# pretool
Parallel remote execution tool - (Yet another parallel ssh/shell)

## Installation:
go get -u github.com/ncode/pretool

## Usage:
```
Parallel remote execution tool - (Yet another parallel ssh/shell)

usage:
    pretool <host1> <host2> <host3>...

Usage:
pretool [flags]

Flags:
    --config string   config file (default is $HOME/.pretool.yaml)
-h, --help            help for pretool
```

Connecting to hosts:
```
pretool host1 host2 host3 host4
pretool(2)>>
```

List connection status:
```
pretool(2)>> :status
Connected hosts (2)
Failed hosts (0)
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
