# pretool
Parallel remote execution tool - (Yet another parallel ssh/shell)

## Installation:
go get -u github.com/ncode/pretool


## Config:
By default it lives in ~/.pretool.yaml

```
username: ncode
groups:
    hosts:
        - host1
        - host2
        - host3
        - host4
```


## Usage:
```
Parallel remote execution tool - (Yet another parallel ssh/shell)

usage:
	pretool <host1> <host2> <host3>...

Usage:
  pretool [flags]

Flags:
      --config string      config file (default is $HOME/.pretool.yaml)
  -h, --help               help for pretool
  -G, --hostGroup string   group of hosts to be loaded from the config file
  -H, --hostsFile string   hosts file to be used instead of the args via stdout (one host per line)
```

Connecting to hosts:
```
pretool host1 host2 host3 host4
pretool(2)>>
Error connection to host host3: Failed to dial: dial tcp: lookup host3: no such host
Error connection to host host4: Failed to dial: dial tcp: lookup host4: no such host
```

Connecting to hostGroups:
```
pretool -G hosts
pretool(2)>>
Error connection to host host3: Failed to dial: dial tcp: lookup host3: no such host
Error connection to host host4: Failed to dial: dial tcp: lookup host4: no such host
```

Connecting to hostsFile:
```
pretool -H /tmp/hosts.txt
pretool(2)>>
Error connection to host host3: Failed to dial: dial tcp: lookup host3: no such host
Error connection to host host4: Failed to dial: dial tcp: lookup host4: no such host
```

List connection status:
```
pretool(2)>> :status
Connected hosts (2)
Failed hosts (2)
```

List hosts:
```
pretool(2)>> :list
host1: Connected(true)
host2: Connected(true)
host3: Connected(false)
host4: Connected(false)
```

Running commands:
```
pretool(2)>> whoami
host1: ncode
host2: ncode
```

## Why do I need it?
pretool is a tool to control interactive shells across multiple hosts from
a single point.

### Motivation
After using [polysh](http://guichaz.free.fr/polysh) for a long time. It came with
the motivation to try to write my own parallel shell in GO. In the end the tool worked 
so well and I decided to open source the code.
