# Namespace executor (nsexec)

## Overview

Run command in specific network Linux namespace ([netns](https://lwn.net/Articles/580893/)). It has the same logic as ```ip netns exec``` but in dedicated binary. It can be useful when you need to run commands as an unprivileged user with using [capabilities](http://man7.org/linux/man-pages/man7/capabilities.7.html):
```
# setcap cap_sys_admin+ep ./nsexec
```
It is possible to use custom configuration files (as using ```ip netns exec```) for specific network namespace stored in ```/etc/netns/NAME/```. For example, you can use custom resolv.conf for *ns1* network namespace placing it as ```/etc/netns/ns1/resolv.conf```.

## Caveats

Each execution creates new mount namespace for custom sysfs with actual information about network devices. If it is undesirable, you should use ```nsenter (1)``` to run a command with namespace(s) of the other process.
