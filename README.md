docker-inspect
==============

Simple http proxy for docker inspect command. 
Useful if you don't want to expose all docker api endpoints to your clients, 
but only containers metadata.

some kind of a `curl http://169.254.169.254/latest/meta-data/` for docker

from a container, with curl available:

```shell
$ docker run -d --name metadata -v /var/run/docker.sock:/var/run/docker.sock yanndegat/docker-inspect
$ docker run --rm --link metadata:metadata alpine sh -c 'apk --update add curl && curl metadata:2204/container/`hostname`'
...
{"Id":"2b077bd686468e5a1965111583884b36fe5515ea9717924a10ef8e4983e5981a","Created":...}%
```

