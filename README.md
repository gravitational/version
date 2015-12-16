# version

`version` is a Go library for automatic build versioning. It attempts to simplify the mundane task of adding build version information to any Go package.

### Usage

Install the command line utility to generate the linker flags necessary for versioning from the cmd/linkflags:

```shell
go install github.com/gravitational/version/cmd/linkflags
```

Add the following configuration to your build script / Makefile
(assuming a bash script):

```bash
GO_LDFLAGS=$(linkflags -pkg=path/to/your/package)

# build with the linker flags:
go build -ldflags="${GO_LDFLAGS}"
```

To use, simply import the package and either obtain the version with `Get` or print the JSON-formatted version with `Print`:

```go
package main

import "github.com/gravitational/version"

func main() {
	version.Print()
}
```
