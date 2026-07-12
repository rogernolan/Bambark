# Bambark deployment

Bambark is a small webhook wrapper that accepts Bambuddy notifications and forwards them to Bark.
It is designed to run on a trusted internal network and does not terminate TLS itself.

## Build

```sh
go build -o bambark ./cmd/bambark
```

## Fresh Debian/systemd setup

```sh
sudo groupadd --system bambark
sudo useradd --system --no-create-home --gid bambark --shell /usr/sbin/nologin bambark

sudo install -o root -g root -m 0755 bambark /usr/local/bin/bambark

sudo install -d -o root -g root -m 0755 /etc/bambark
sudo install -o root -g root -m 0640 /dev/null /etc/bambark/bambark.env
sudoedit /etc/bambark/bambark.env

sudo install -o root -g root -m 0644 deploy/bambark.service /etc/systemd/system/bambark.service
sudo systemctl daemon-reload
sudo systemctl enable --now bambark.service
```

Use placeholders in `/etc/bambark/bambark.env` until you replace them with your real settings:

```dotenv
LISTEN_ADDR=:8080
BARK_URL=http://127.0.0.1:8081
BARK_DEVICE_KEY=replace-with-bark-device-key
WEBHOOK_BEARER_TOKEN=replace-with-a-long-random-token
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

The preferred method is `POST`. For compatibility with webhook clients that only
support `GET`, Bambark also accepts `GET` requests with `title` and `message`
query parameters:

```sh
curl -G http://<lxc-address>:8080/webhook/bambuddy \
  -H 'Authorization: Bearer <webhook-bearer-token>' \
  --data-urlencode 'title=Print finished' \
  --data-urlencode 'message=Tray 1 is ready'
```

The wrapper assumes the request comes from the trusted internal network and does not provide TLS termination.

## Check the service

```sh
curl http://127.0.0.1:8080/healthz
```

If the service is running, the health check returns `ok`.

## Inspect service logs

```sh
sudo journalctl -u bambark.service -f
```

Use this to follow the systemd service logs while testing or debugging.

## Rootless redeploy

For a user-level systemd deployment, copy and edit the environment file once:

```sh
mkdir -p ~/.config/bambark
install -m 0600 deploy/bambark.env.example ~/.config/bambark/bambark.env
$EDITOR ~/.config/bambark/bambark.env
```

If the service must run when you are not logged in, an administrator must enable
lingering once:

```sh
sudo loginctl enable-linger "$USER"
```

Then redeploy from the repository with:

```sh
./scripts/deploy.sh
```

The script refuses to run as root. It builds to `~/.local/bin/bambark`, installs
the user service under `~/.config/systemd/user/`, and reloads, enables, and
restarts `bambark.service` through `systemctl --user`.
