# Containerised nodes

`deploy/update.sh` deliberately refuses to touch a node running in a
container. The path it would find in `/proc/<pid>/exe` belongs to the
container's mount namespace, and a binary swapped into a running container is
undone the moment that container is recreated from its image. A container is
updated by rebuilding the image, not by replacing a file inside it.

These two Dockerfiles build an image from a **released binary**. The ones at
the repository root compile from source instead, which needs a Go toolchain
and working DNS inside the build -- on a small node that is two more things
that can fail. Pick whichever suits; nothing downstream can tell the
difference.

## Build

Architecture matters: Oracle's ARM instances are common enough that guessing
amd64 will bite you.

```sh
case $(uname -m) in
  x86_64)        A=amd64 ;;
  aarch64|arm64) A=arm64 ;;
  *) echo "no release for $(uname -m)"; exit 1 ;;
esac
TAG=v0.27.13-hz1
BASE=https://github.com/hector918/nps/releases/download/$TAG

# client
curl -fsSLO $BASE/linux_${A}_client.tar.gz
curl -fsSLO $BASE/sha256sums.txt
sha256sum -c --ignore-missing sha256sums.txt      # do not skip
tar -xzOf linux_${A}_client.tar.gz npc > npc && chmod +x npc
docker build -f Dockerfile.npc -t npc:$TAG .

# server: the binary is not enough, web/ ships with it
tar -xzf linux_${A}_server.tar.gz nps web
docker build -f Dockerfile.nps -t nps:$TAG .
```

Only the binary comes out of the client tarball on purpose. It also contains
`conf/npc.conf`, a template -- baking that in, or unpacking it over a node,
replaces a real vkey and tunnel list with placeholders.

## Run

```sh
docker run -d --name npc \
  --restart unless-stopped \
  -v /path/to/your/npc.conf:/conf/npc.conf:ro \
  npc:v0.27.13-hz1
```

The config is a bind mount, never an image layer: it is the node's identity.
`ENTRYPOINT` is the binary and `CMD` is `-config=/conf/npc.conf`, so a
different path is just a different argument:

```sh
docker run ... npc:v0.27.13-hz1 -config=/etc/npc/npc.conf
```

The server needs its state directory mounted at `/conf` -- `nps.conf` plus
`clients.json`, `hosts.json`, `tasks.json` and the certificate pair. Those are
state, not image content.

## Update

Build the new image, then recreate the container. **If the container is
managed by compose, use compose** -- recreating it with `docker run` takes it
out of compose's management, and the next `docker compose up` will start a
second one alongside it. Two clients sharing a vkey fight over the tunnel.

```sh
docker inspect <name> --format '{{json .Config.Labels}}'   # compose labels?
```

Compose: point `image:` at the new tag, then `docker compose up -d <service>`.
Otherwise: `docker rm -f <name>` and run again with the same flags, which
`docker inspect` will tell you:

```sh
docker inspect <name> --format 'Entrypoint={{json .Config.Entrypoint}}
Cmd={{json .Config.Cmd}}
Binds={{json .HostConfig.Binds}}
Network={{.HostConfig.NetworkMode}}
Ports={{json .HostConfig.PortBindings}}
Restart={{.HostConfig.RestartPolicy.Name}}'
```

Keep the old image until the new container has proven itself; rolling back is
then just the previous tag.

## Two things that are easy to miss

**The version has to carry the fork marker.** The server decides whether a
client understands the control messages this fork added by looking for `-hz`
in the version it reports; an older client would misread them and lose its
control channel. A binary taken from a release already carries it. One built
from source without `-ldflags -X ...version.VERSION=<tag>` reports `dev`, and
the server will refuse to push updates to that node.

**Server-pushed updates do not apply to containers.** The in-process updater
replaces the binary and restarts into it, which a container throws away on the
next recreate. Update containerised nodes through their images.

## If the build cannot resolve DNS

`docker build --network=host` is usually the fastest way past a broken bridge
network. The release-based Dockerfiles here only need network access for the
`apk add ca-certificates` line, so this comes up less than with the
source-building ones at the repository root.
