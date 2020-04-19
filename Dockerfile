FROM golang:1.13.10 as build

WORKDIR /work

ADD go.mod go.sum /work/

RUN go mod download

ADD main.go /work/
ADD cmd /work/cmd/

RUN CGO_ENABLED=0 GOOS=linux go build -o hcloud-metallb-floater .

FROM alpine:3.11

RUN apk add --update --no-cache ca-certificates

COPY --from=build /work/hcloud-metallb-floater /hcloud-metallb-floater

ENTRYPOINT /hcloud-metallb-floater
