# Bambark deployment

Bambark is a small webhook wrapper that accepts Bambuddy notifications and forwards them to Bark.
It is designed to run on a trusted internal network and does not terminate TLS itself.

## Build

```sh
go build -o bambark ./cmd/bambark
```

## Install

```sh
sudo install -m 0755 bambark /usr/local/bin/bambark
sudo install -d -m 0750 /etc/bambark
sudo install -m 0640 deploy/bambark.env.example /etc/bambark/bambark.env
sudo systemctl enable --now bambark.service
```

## Configure Bambuddy

Use this webhook URL in Bambuddy:

```text
http://<lxc-address>:8080/webhook/bambuddy
```

Send notifications with a bearer token in the `Authorization` header:

```sh
curl -X POST http://<lxc-address>:8080/webhook/bambuddy \
  -H 'Authorization: Bearer <webhook-bearer-token>' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Print finished","message":"Tray 1 is ready"}'
```

The wrapper assumes the request comes from the trusted internal network and does not provide TLS termination.

## Check the service

```sh
curl http://127.0.0.1:8080/healthz
```

If the service is running, the health check returns `ok`.
