package cmd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hetznercloud/hcloud-go/hcloud"
	zapcore "go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kubeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const annotationMetalLBLayer2OwnerNode = "layer2.metallb.universe.tf/owner-node"

func run(args []string) error {
	lvl := zapcore.NewAtomicLevelAt(zapcore.InfoLevel)
	if rootFlags.Verbose {
		lvl.SetLevel(zapcore.DebugLevel)
	}
	logf.SetLogger(zap.New(zap.Level(&lvl)))

	var log = logf.Log.WithName(appName)

	hcloudToken, err := getEnvRequired("HCLOUD_TOKEN")
	if err != nil {
		return err
	}
	hcloudClient := hcloud.NewClient(hcloud.WithToken(hcloudToken))

	mgr, err := manager.New(kubeconfig.GetConfigOrDie(), manager.Options{
		LeaderElection:          rootFlags.EnableLeaderElection,
		LeaderElectionID:        "hcloud-metallb-floater-controller",
		LeaderElectionNamespace: "kube-system",
		MetricsBindAddress:      rootFlags.MetricsAddr,
		SyncPeriod:              &rootFlags.SyncPeriod,
	})
	if err != nil {
		return fmt.Errorf("cloud not create manager: %w", err)
	}

	err = builder.
		ControllerManagedBy(mgr).
		WithOptions(controller.Options{}).
		For(&corev1.Service{}).
		Complete(&ServiceReconciler{
			Log:          log.WithName("service-reconciler"),
			hcloudClient: hcloudClient,
		})
	if err != nil {
		return fmt.Errorf("cloud not create controller: %w", err)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		return fmt.Errorf("cloud not start manager: %w", err)
	}

	return nil
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// ServiceReconciler is a simple ControllerManagedBy example implementation.
type ServiceReconciler struct {
	client.Client
	Log          logr.Logger
	hcloudClient *hcloud.Client
}

// Reconcile determines if a service is used by metallb in level2 mode and the
// acts accordingly
func (r *ServiceReconciler) Reconcile(req reconcile.Request) (reconcile.Result,
	error) {
	ctx := context.TODO()
	log := r.Log.WithValues("service", req.NamespacedName)

	// Read the Service
	svc := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
		return reconcile.Result{}, err
	}

	// Check for the annotation
	if svc.ObjectMeta.Annotations == nil {
		return reconcile.Result{}, nil
	}
	nodeName, ok := svc.ObjectMeta.Annotations[annotationMetalLBLayer2OwnerNode]
	if !ok {
		return reconcile.Result{}, nil
	}

	// Get the loadbalancer IP if assigned
	var ip net.IP
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.IP == "" {
			continue
		}
		ip = net.ParseIP(ing.IP)
		if ip == nil {
			return reconcile.Result{}, fmt.Errorf("invalid Loadbalancer IP: '%s'", ing.IP)
		}
		break
	}
	if ip == nil {
		return reconcile.Result{}, fmt.Errorf("no Loadbalancer IP attached")
	}
	log = log.WithValues("node", nodeName, "floating_ip", ip)

	// Find the node the service is attached to
	node := &corev1.Node{}
	if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		return reconcile.Result{}, err
	}

	hcloudPrefix := "hcloud://"
	if node.Spec.ProviderID == "" {
		return reconcile.Result{}, fmt.Errorf(
			"providerID of node '%s' is not set",
			node.Name,
		)

	} else if !strings.HasPrefix(node.Spec.ProviderID, hcloudPrefix) {
		return reconcile.Result{}, fmt.Errorf(
			"providerID '%s' of node '%s' has no '%s' prefix",
			node.Spec.ProviderID,
			node.Name,
			hcloudPrefix,
		)
	}

	serverID, err := strconv.ParseInt(node.Spec.ProviderID[len(hcloudPrefix):], 10, 32)
	if err != nil {
		return reconcile.Result{}, err
	}

	floatingIP, err := r.findFloatingIPByIP(ctx, ip)
	if err != nil {
		return reconcile.Result{}, err
	}
	log = log.WithValues("node_id", serverID, "floating_ip_id", floatingIP.ID)

	if floatingIP.Server != nil && floatingIP.Server.ID == int(serverID) {
		log.V(1).Info("floatingIP already points to the correct node")
		return reconcile.Result{}, nil
	}

	if _, _, err := r.hcloudClient.FloatingIP.Assign(
		ctx,
		floatingIP,
		&hcloud.Server{ID: int(serverID)},
	); err != nil {
		return reconcile.Result{}, err
	}
	log.Info("successfully assign floatingIP to correct node")

	return reconcile.Result{}, nil
}

// findFloatingIP by service
func (r *ServiceReconciler) findFloatingIPByIP(ctx context.Context, ip net.IP) (*hcloud.FloatingIP, error) {
	// TODO: This is probably quite expensive and misses FloatingIP if there are exceeding the page limit
	floatingIPs, err := r.hcloudClient.FloatingIP.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to list floating IPs in the api: %s", err)
	}

	for _, f := range floatingIPs {
		if f.Type == hcloud.FloatingIPTypeIPv4 {
			if ip.Equal(f.IP) {
				return f, nil
			}
		}
		if f.Type == hcloud.FloatingIPTypeIPv6 {
			if f.Network != nil && f.Network.Contains(ip) {
				return f, nil
			}
		}
	}
	return nil, fmt.Errorf("No floating IP with IP %s found", ip)
}

func (r *ServiceReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
