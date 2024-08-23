# Setup

## Overview

To install tootik, you need to:

* Get a server and point a domain to it
* Generate a valid HTTPS certificate (we're using a free [Let's Encrypt](https://letsencrypt.org/) certificate) and a Gemini certificate
* Manually run tootik on your server (we're using a prebuilt static executable for x86 or ARM)
* Verify that the Gemini frontend is accessible and works
* Verify that federation works in both directions
* Stop tootik and configure your machine to keep it running at all times

## Steps

1. Set up a server with a public address; preferably, both IPv4 and IPv6.

The $5/mo nodes at [Linode](https://www.linode.com/lp/refer/?r=2e06503874831fe7912dc300e1e6f759e7944ea1) (**disclaimer: referral link**) should be more than enough to run tootik, but this guide should work on any [Debian](https://www.debian.org/) 11 or 12 machine. (If you wish to help fund the development of tootik, use the referral link. Thank you!)

2. Register a domain and make sure your domain points to the server. Wait until `getent hosts $domain` or `nslookup $domain` returns the server's public address.

[gen.xyz](https://gen.xyz/number) offers cheap domains but you can use any domain as long as it's not blocked by other ActivityPub-compatible servers or has bad reputation.

**In every command that appears in this guide, replace `$domain` with your domain** or simply run `domain=` followed by your domain now.

3. Install [Certbot](https://certbot.eff.org/) and generate a HTTPS certificate for federation:

```
apt update
apt install snapd
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot
certbot certonly --standalone
```

4. Create a directory for tootik files and copy the certificate:

```
mkdir /tootik-cfg
cp /etc/letsencrypt/live/*/fullchain.pem /tootik-cfg/https-cert.pem
cp /etc/letsencrypt/live/*/privkey.pem /tootik-cfg/https-key.pem
```

5. Create a post-renewal hook that updates the copied certificate on renewal:

```
cat << EOF > /etc/letsencrypt/renewal-hooks/deploy/tootik.sh
#!/bin/sh

cp -f /etc/letsencrypt/live/*/fullchain.pem /tootik-cfg/https-cert.pem
cp -f /etc/letsencrypt/live/*/privkey.pem /tootik-cfg/https-key.pem
EOF
chmod 755 /etc/letsencrypt/renewal-hooks/deploy/tootik.sh
```

tootik monitors these files for changes and should restart its HTTPS listener automatically every time the certificate is renewed and the hook replaces the files.

6. Generate a self-signed TLS certificate for the Gemini frontend

```
openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=$domain" -out /tootik-cfg/gemini-cert.pem -keyout /tootik-cfg/gemini-key.pem -days 3650
```

7. Download the [Garden Fence](https://github.com/gardenfence/blocklist) blocklist:

```
curl -L https://github.com/gardenfence/blocklist/raw/main/gardenfence-mastodon.csv > /tootik-cfg/gardenfence-mastodon.csv
```

8. Create an unprivileged user and a separate directory for the tootik database, then download tootik and run it:

```
mkdir /tootik-data
useradd -mr tootik
chown -R tootik:tootik /tootik-cfg /tootik-data
curl -L https://github.com/dimkr/tootik/releases/latest/download/tootik-$(case `uname -m` in x86_64) echo amd64;; aarch64) echo arm64;; i686) echo 386;; armv7l) echo arm;; esac) -o /usr/local/bin/tootik
chmod 755 /usr/local/bin/tootik
tootik -domain $domain -addr :443 -gemaddr :1965 -gopheraddr :70 -fingeraddr :79 -blocklist /tootik-cfg/gardenfence-mastodon.csv -cert /tootik-cfg/https-cert.pem -key /tootik-cfg/https-key.pem -gemcert /tootik-cfg/gemini-cert.pem -gemkey /tootik-cfg/gemini-key.pem -db /tootik-data/db.sqlite3
```

To enable more verbose logging, add `-loglevel -4`.

We use a separate directory for the database because tootik monitors the directory that contains the HTTPS certificate and the directory that contains the blocklist for changes, so it can reload these files when they get replaced or modified. The database changes often, so putting the database in the same directory as the files tootik monitors for changes can result in many wakeups and increased CPU usage.

**tootik writes logs to stderr. Keep this shell open for troubleshooting purposes, and continue in another.**

9. From a remote machine, verify that tootik is accessible over HTTPS:

```
curl -v https://$domain
```

Output should contain:

```
< HTTP/2 301
< location: gemini://$domain
```

If `curl` times out, check your server's firewall: port 443 is probably blocked.

10. Verify that tootik is accessible from a remote machine over Gemini, with any Gemini client.

If you have a graphical web browser and a Gemini client that configures itself as the default handler for gemini:// URLs, opening https://$domain through the web browser should display a popup that asks you to use the Gemini client instead. Otherwise, fire up your Gemini client and navigate to gemini://$domain.

11. Register by creating a client certificate or clicking "Sign in" and use "View profile" to verify that your instance is able to "discover" users on other servers.

Once a user is discovered, you can follow this user and your instance should start receiving new posts by this user. They should appear under your user's inbox ("My feed") after a while (`FeedUpdateInterval`) and the user's profile.

**If you don't see any posts, check tootik's output.**

If certificate validation fails for all outgoing requests, try to update CA certificates using `apt update && apt-get install --only-upgrade ca-certificates` and synchronize the server's clock using `apt install systemd-timesyncd && timedatectl set-ntp true`.

12. Repeat the same check in the other direction: try to search for your user in your tootik instance (`$user@$domain`) from another ActivityPub-compatible server, then follow it and check if the other server receives new posts by your tootik user.

**If you don't see any posts, check tootik's output.**

13. Ask tootik to stop using CTRL+c and wait.

14. Add a systemd unit for tootik, to make it run at startup, restart it if it crashes, and save its log on disk (with log rotation):

```
cat << EOF > /etc/systemd/system/tootik.service
[Unit]
Description=tootik
After=network.target

[Service]
ExecStart=tootik -domain $domain -addr :443 -gemaddr :1965 -gopheraddr :70 -fingeraddr :79 -blocklist /tootik-cfg/gardenfence-mastodon.csv -cert /tootik-cfg/https-cert.pem -key /tootik-cfg/https-key.pem -gemcert /tootik-cfg/gemini-cert.pem -gemkey /tootik-cfg/gemini-key.pem -db /tootik-data/db.sqlite3
User=tootik
Group=tootik
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
Restart=always

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable tootik
systemctl start tootik
```

Now you can view tootik logs using `journalctl`, e.g.:

```
journalctl -u tootik -S '5 minutes ago' -f
```

To stop tootik:

```
systemctl stop tootik
```

To start tootik:

```
systemctl start tootik
```

To check the tootik version:

```
tootik -version
```

To disable new user registration, add `-closed` to the tootik command-line in `ExecStart` and restart it:

```
sed -i 's/^ExecStart=.*/& -closed/' /etc/systemd/system/tootik.service
systemctl daemon-reload
systemctl restart tootik
```

To disable HTTPS and make tootik speak HTTP instead (useful if you wish to run tootik behind a HTTPS reverse proxy), add `-plain` to the tootik command-line in `ExecStart` and restart it:

```
sed -i 's/^ExecStart=.*/& -plain/' /etc/systemd/system/tootik.service
systemctl daemon-reload
systemctl restart tootik
```

To view and change tootik configuration defaults:

```
tootik -dumpcfg > /tootik-cfg/cfg.json
# edit /tootik-cfg/cfg.json
sed -i 's~^ExecStart=.*~& -cfg /tootik-cfg/cfg.json~' /etc/systemd/system/tootik.service
systemctl daemon-reload
systemctl restart tootik
```

To update and restart tootik:

```
systemctl stop tootik
curl -L https://github.com/dimkr/tootik/releases/latest/download/tootik-$(case `uname -m` in x86_64) echo amd64;; aarch64) echo arm64;; i686) echo 386;; armv7l) echo arm;; esac) -o /usr/local/bin/tootik
systemctl start tootik
```

To add a community, then set its bio and avatar:

```
tootik -domain $domain -db /tootik-data/db.sqlite3 add-community fountainpens
# put bio in /tmp/bio.txt
tootik -domain $domain -db /tootik-data/db.sqlite3 set-bio fountainpens /tmp/bio.txt
# put avatar in /tmp/avatar.png
tootik -domain $domain -db /tootik-data/db.sqlite3 set-avatar fountainpens /tmp/avatar.png
```

## Running behind a reverse proxy

* Run tootik with `-plain`, so it speaks HTTP and the reverse proxy handles TLS.
   * Ensure the reverse proxy uses a valid TLS certificate
* Use `-addr` (i.e. `-addr 127.0.0.1:8080`) to specify the port used by tootik's HTTP listener.
* Use `-domain` to specify the external host and port combination other servers use to talk to your instance:
   * If tootik runs on `example.com` with `-addr 127.0.0.1:8080 -plain` with a reverse proxy on port 443, pass `-domain example.com`
   * If tootik runs on `example.com` with `-addr 127.0.0.1:8080 -plain` with a reverse proxy on port 8443, pass `-domain example.com:8443`
* Forward requests from the reverse proxy to tootik.
   * Preserve the `Signature` header when forwarding POST requests to `/inbox/$user`, otherwise tootik cannot validate incoming requests
   * Preserve the `Collection-Synchronization` header when forwarding POST requests to `/inbox/$user` if you want follower synchronization to work (recommended)

## Troubleshooting

* If tootik's HTTPS listener uses a port other than 443 (say, tootik runs with `-addr :8888`) and this is the port other instances use to talk to tootik, `-domain` must include the port (for example, `-domain example.com:8888`).
* If tootik is behind a proxy, make sure the proxy passes the `Signature` header to tootik.
* grep logs for `actor is too young` and decrease `MinActorAge` if the federated account you're trying to talk to is newly registered.
