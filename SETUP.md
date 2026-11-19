# Setup

## Overview

To install tootik, you need to:

* Get a server and point a domain to it
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

3. Create a directory for tootik files:

```
mkdir /tootik-cfg
```

If you already have a Gemini certificate that you wish to use, place the certificate in `/tootik-cfg/gemini-cert.pem` and the private key in `/tootik-cfg/gemini-key.pem`, then run tootik with `-gemcert /tootik-cfg/gemini-cert.pem -gemkey /tootik-cfg/gemini-key.pem`. Otherwise, tootik generates a self-signed `secp256r1` certificate with a 10 year lifespan.

If you already have a HTTPS certificate that you wish to use, place the certificate in `/tootik-cfg/https-cert.pem` and the private key in `/tootik-cfg/https-key.pem`, then run tootik with `-cert /tootik-cfg/https-cert.pem -key /tootik-cfg/https-key.pem`; tootik monitors the directories containing these files for changes and should restart its HTTPS listener automatically once they get replaced on renewal. Therefore, putting these files in the same directory as the database or the blocklist can result in many wakeups and increased CPU usage. If you don't have a HTTPS certificate, tootik falls back to generating one using [Let's Encrypt](https://letsencrypt.org/) and the `tls-alpn-01` ACME challenge type, while accepting its Terms of Service, then caches the certificate in the database and renews it automatically.

4. Download the [Garden Fence](https://github.com/gardenfence/blocklist) blocklist:

```
curl -L https://github.com/gardenfence/blocklist/raw/main/gardenfence-mastodon.csv > /tootik-cfg/gardenfence-mastodon.csv
```

5. Install PostgreSQL, create a database named `tootik` owned by a role named `tootik`, set up [pg_lake](#pg_lake), then create an unprivileged user, download tootik and run it:

```
useradd -mr tootik
chown -R tootik:tootik /tootik-cfg
curl -L https://github.com/dimkr/tootik/releases/latest/download/tootik-$(case `uname -m` in x86_64) echo amd64;; aarch64) echo arm64;; i686) echo 386;; armv7l) echo arm;; esac) -o /usr/local/bin/tootik
chmod 755 /usr/local/bin/tootik
tootik -domain $domain -blocklist /tootik-cfg/gardenfence-mastodon.csv -db postgres:///tootik?host=/run/postgresql -lake postgres://localhost:5433/tootik
```

To enable more verbose logging, add `-loglevel -4`.

To serve read-only pages from a PostgreSQL streaming read replica, add `-replica` followed by the replica's DSN.

**tootik writes logs to stderr. Keep this shell open for troubleshooting purposes, and continue in another.**

6. From a remote machine, verify that tootik is accessible over HTTPS:

```
curl -v https://$domain
```

Output should contain:

```
< HTTP/2 301
< location: gemini://$domain
```

If `curl` times out, check your server's firewall: port 443 is probably blocked.

7. Verify that tootik is accessible from a remote machine over Gemini, with any Gemini client.

If you have a graphical web browser and a Gemini client that configures itself as the default handler for gemini:// URLs, opening https://$domain through the web browser should display a popup that asks you to use the Gemini client instead. Otherwise, fire up your Gemini client and navigate to gemini://$domain.

8. Register by creating a client certificate or clicking "Sign in" and use "View profile" to verify that your instance is able to "discover" users on other servers.

Once a user is discovered, you can follow this user and your instance should start receiving new posts by this user. They should appear under your user's inbox ("My feed") after a while (`FeedUpdateInterval`) and the user's profile.

**If you don't see any posts, check tootik's output.**

If certificate validation fails for all outgoing requests, try to update CA certificates using `apt update && apt-get install --only-upgrade ca-certificates` and synchronize the server's clock using `apt install systemd-timesyncd && timedatectl set-ntp true`.

9. Repeat the same check in the other direction: try to search for your user in your tootik instance (`$user@$domain`) from another ActivityPub-compatible server, then follow it and check if the other server receives new posts by your tootik user.

**If you don't see any posts, check tootik's output.**

10. Ask tootik to stop using CTRL+c and wait.

11. Write tootik's defaults to a configuration file that can be edited later:

```
tootik -dumpcfg > /tootik-cfg/cfg.json
```

12. Add a systemd unit for tootik, to make it run at startup, restart it if it crashes, and save its log on disk (with log rotation):

```
cat << EOF > /etc/systemd/system/tootik.service
[Unit]
Description=tootik
After=network.target

[Service]
ExecStart=tootik -domain $domain -blocklist /tootik-cfg/gardenfence-mastodon.csv -db postgres:///tootik?host=/run/postgresql -lake postgres://localhost:5433/tootik -cfg /tootik-cfg/cfg.json
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

To enable invite-only user registration:

```
jq '.RequireInvitation = true' /tootik-cfg/cfg.json > /tmp/cfg.json
mv -f /tmp/cfg.json /tootik-cfg/cfg.json
systemctl restart tootik
```

To disable user registration, enable invite-only registration, then forbid creation of invitations:

```
jq '.RequireInvitation = true | .MaxInvitationsPerUser = 0' /tootik-cfg/cfg.json > /tmp/cfg.json
mv -f /tmp/cfg.json /tootik-cfg/cfg.json
systemctl restart tootik
```

To disable HTTPS and make tootik speak HTTP instead (useful if you wish to run tootik behind a HTTPS reverse proxy), add `-plain` to the tootik command-line in `ExecStart` and restart it:

```
sed -i 's/^ExecStart=.*/& -plain/' /etc/systemd/system/tootik.service
systemctl daemon-reload
systemctl restart tootik
```

To restrict access to registered users:

```
jq '.RequireRegistration = true' /tootik-cfg/cfg.json > /tmp/cfg.json
mv -f /tmp/cfg.json /tootik-cfg/cfg.json
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
tootik -domain $domain -db postgres:///tootik?host=/run/postgresql add-community fountainpens
# put bio in /tmp/bio.txt
tootik -domain $domain -db postgres:///tootik?host=/run/postgresql set-bio fountainpens /tmp/bio.txt
# put avatar in /tmp/avatar.png
tootik -domain $domain -db postgres:///tootik?host=/run/postgresql set-avatar fountainpens /tmp/avatar.png
```

To run an interactive shell:

```
tootik -domain $domain -db postgres:///tootik?host=/run/postgresql shell
```

To run an interactive shell on behalf of a user named `alice`:

```
tootik -domain $domain -db postgres:///tootik?host=/run/postgresql shell alice
```

## pg_lake

tootik computes the statistics shown in the status page from [Iceberg](https://iceberg.apache.org/) tables, through a separate PostgreSQL server with [pg_lake](https://github.com/Snowflake-Labs/pg_lake). This server holds a logical replica of the columns these statistics need, and stores the Iceberg tables in S3-compatible storage, like [RustFS](https://rustfs.com/).

1. Allow logical replication from the primary, and let the role pg_lake connects as use it:

```
psql -c "ALTER SYSTEM SET wal_level = logical" -c "ALTER ROLE tootik REPLICATION"
systemctl restart postgresql
```

2. Run RustFS and create a bucket:

```
docker run -d --name rustfs --restart always -p 127.0.0.1:9000:9000 -v rustfs:/data -e RUSTFS_ACCESS_KEY=$access_key -e RUSTFS_SECRET_KEY=$secret_key -e RUSTFS_CONSOLE_ENABLE=false rustfs/rustfs:1.0.6
docker exec rustfs rc alias set local http://127.0.0.1:9000 $access_key $secret_key
docker exec rustfs rc bucket create local/tootik
```

3. Build pg_lake and pgduck_server, as described in the [pg_lake documentation](https://github.com/Snowflake-Labs/pg_lake/blob/main/docs/building-from-source.md), and run PostgreSQL on port 5433 with `shared_preload_libraries = 'pg_extension_base'`. Start pgduck_server with an `--init_file_path` that lets it access RustFS:

```
CREATE SECRET rustfs (TYPE s3, SCOPE 's3://tootik', USE_SSL false, KEY_ID '...', SECRET '...', URL_STYLE 'path', ENDPOINT 'localhost:9000', REGION 'us-east-1');
```

4. Create a database named `tootik` on the pg_lake server, owned by a superuser role that tootik connects as, and tell pg_lake where to store Iceberg tables:

```
psql -p 5433 -c "CREATE DATABASE tootik" -c "ALTER DATABASE tootik SET pg_lake_iceberg.default_location_prefix = 's3://tootik/iceberg'"
```

On startup, tootik creates the tables it needs in this database, and subscribes to the primary: by default, pg_lake connects to the primary using the `-db` DSN, and `-lakepublisher` overrides it. The subscription and its replication slot on the primary are named after the pg_lake database.

Every export rewrites the Iceberg tables. pg_lake deletes the replaced files once they are older than `pg_lake_engine.orphaned_file_retention_period` (10 days by default): lower it to use less storage.

## Running behind a reverse proxy

* Run tootik with `-plain`, so it speaks HTTP and the reverse proxy handles TLS.
   * Ensure the reverse proxy uses a valid TLS certificate
* Use `-addr` (i.e. `-addr 127.0.0.1:8080`) to specify the port used by tootik's HTTP listener.
* Use `-domain` to specify the external host and port combination other servers use to talk to your instance:
   * If tootik runs on `example.com` with `-addr 127.0.0.1:8080 -plain` with a reverse proxy on port 443, pass `-domain example.com`
   * If tootik runs on `example.com` with `-addr 127.0.0.1:8080 -plain` with a reverse proxy on port 8443, pass `-domain example.com:8443`
* Forward requests from the reverse proxy to tootik.
   * Preserve the `Signature` and `Signature-Input` headers when forwarding POST requests to `/inbox/$user`, otherwise tootik cannot validate incoming requests
   * Raise the buffer size if needed: `ml-dsa-44`-signed requests can carry ~8K of headers
   * Preserve the `Collection-Synchronization` header when forwarding POST requests to `/inbox/$user` if you want follower synchronization to work (recommended)

## Troubleshooting

* If tootik's HTTPS listener uses a port other than 443 (say, tootik runs with `-addr :8888`) and this is the port other instances use to talk to tootik, `-domain` must include the port (for example, `-domain example.com:8888`).
* If tootik is behind a proxy, make sure the proxy passes the `Signature` and `Signature-Input` headers to tootik.
* grep logs for `actor is too young` and decrease `MinActorAge` if the federated account you're trying to talk to is newly registered.

## Restricting SSH Access

To protect the server and the user data on it, it's recommended to restrict SSH access.

First, change the SSH port from the default of 22 to something else:

```
sed -i 's/^#Port 22/Port 7676/' /etc/ssh/sshd_config
systemctl restart ssh.service
```

Then, hide this listening port from scanners using [port knocking](https://wiki.archlinux.org/title/Port_knocking):

```
for x in iptables ip6tables; do
    $x -N ssh
    $x -A INPUT -p tcp --dport 7676 -j ssh
    $x -A ssh -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
    $x -A ssh -p tcp --sport 1234 -m recent --name knock1 --set -j DROP
    $x -A ssh -m conntrack --ctstate NEW -m recent --name knock1 \! --rcheck --seconds 50 -j DROP
    $x -A ssh -p tcp --sport 2345 -m recent --name knock2 --set -j DROP
    $x -A ssh -m conntrack --ctstate NEW -m recent --name knock2 \! --rcheck --seconds 40 -j DROP
    $x -A ssh -p tcp --sport 3456 -m recent --name knock3 --set -j DROP
    $x -A ssh -m conntrack --ctstate NEW -m recent --name knock3 \! --rcheck --seconds 30 -j DROP
    $x -A ssh -j ACCEPT
done
```

To connect:

```
for x in 1234 2345 3456; do nc -vw1 -p $x example.com 7676; done
ssh -p 7676 root@example.com
```

To reapply these SSH port restrictions after reboot:

```
apt install iptables-persistent
iptables-save > /etc/iptables/rules.v4
ip6tables-save > /etc/iptables/rules.v6
```

## Rate Limiting

tootik handles /robots.txt requests over both HTTP and [Gemini](gemini://geminiprotocol.net/docs/companion/robots.gmi): it asks all kinds of crawlers to leave it alone. However, some crawlers don't obey what robots.txt says and flood the server with requests. They visit all posts they can find, discover users (post authors, reply authors and mentioned users), then view the profiles of these users to discover more posts and more users.

Therefore, it's recommended to have some kind of rate limiting for Gemini requests:

```
for x in iptables ip6tables; do
   $x -N ratelimit
   $x -A ratelimit -j LOG --log-prefix "rate limit "
   $x -A ratelimit -j DROP
   $x -I INPUT -p tcp --dport 1965 -m conntrack --ctstate NEW -m hashlimit --hashlimit-name rate --hashlimit-mode srcip --hashlimit-above 100/hour -j ratelimit
done
```

To reapply rate limiting after reboot:

```
apt install iptables-persistent
iptables-save > /etc/iptables/rules.v4
ip6tables-save > /etc/iptables/rules.v6
```
