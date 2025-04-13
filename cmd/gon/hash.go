package main

import (
	"fmt"

	"github.com/alecthomas/kingpin"
	"github.com/bas-es/go-nix/pkg/nixhash"
)

var (
	hashCmd  = kingpin.Command("hash", "Hash a path.")
	hashPath = hashCmd.Arg("path", "path").Required().String()
	hashName = hashCmd.Flag("name", "Name in store.").Short('n').String()
)

var hashMain = register("hash", func() {
	fmt.Println(nixhash.StorePath(*hashPath, *hashName))
})
