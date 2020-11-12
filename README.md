## operator

A simple library for executing commands and scripts on a remote host via SSH.

`go get github.com/jsiebens/operator`

## Example

This example uploads a given script to a remote host and executes it:

```golang
package main

import (
	"flag"
	"fmt"
	"github.com/jsiebens/operator"
	"math/rand"
	"os"
	"time"
)

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func main() {

	user := flag.String("user", "root", "Username for SSH login (default \"root\")")
	host := flag.String("host", "", "Target host")
	port := flag.Int("port", 22, "The port on which to connect for ssh (default 22)")
	pwd := flag.String("password", "", "The password to use for remote login (optional)")
	privateKey := flag.String("private-key", "", "The ssh key to use for remote login (optional)")
	script := flag.String("script", "", "The script to executed on the remote host")

	flag.Parse()

	if *host == "" || *script == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	callback := func(op operator.CommandOperator) error {
		remoteFile := fmt.Sprintf("/tmp/script.%d", seededRand.Int())
		err := op.UploadFile(*script, remoteFile, "0755")
		if err != nil {
			return err
		}
		op.Execute(remoteFile)
		return nil
	}

	var err error
	if *privateKey != "" {
		err = operator.ExecuteRemoteWithPrivateKey(*host, *port, *user, *privateKey, callback)
	} else if *pwd != "" {
		err = operator.ExecuteRemoteWithPassword(*host, *port, *user, *pwd, callback)
	} else {
		err = operator.ExecuteRemote(*host, *port, *user, callback)
	}

	if err != nil {
		fmt.Printf("error: %s \n\n", err)
	}
}
```

## Contributing

Commits must be signed off with `git commit -s`

License: MIT