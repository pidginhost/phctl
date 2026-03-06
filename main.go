package main

import "github.com/pidginhost/phctl/cmd"

var version = "dev"

func init() {
	cmd.SetVersion(version)
}

func main() {
	cmd.Execute()
}
