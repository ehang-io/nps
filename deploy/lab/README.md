# Two-container lab

`run.sh` builds the old release (a tag, `v0.27.11-hz1` by default) and the
current tree, puts a server in one container and the clients in another on a
docker network with no way out, and runs the cutover questions through them:

1. old server, old clients: baseline
2. new server, new clients: works with nothing configured but the vkey
3. old server, new clients: refused, clients keep retrying
4. new server, old clients: refused, clients keep retrying
5. an update pushed from the old server to the incompatible release: the node
   rolls itself back after the 60 s verification window
6. the cutover order, clients first and then the server: the tunnel is back
   seconds after the server switches
7. a man in the middle that terminates TLS and relays every byte to the real
   server: refused, and the server raises an alert
8. the same vkey from a second address: the server raises an alert

The client container answers for github.com with a fake release server so the
self-updater can be exercised without a network. Nothing touches the host's
own nps, npc or /etc/nps.

    deploy/lab/run.sh [old-tag]
