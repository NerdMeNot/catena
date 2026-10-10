// Command catenalint runs the catena checks.
//
// As a vet tool:
//
//	go vet -vettool=$(which catenalint) ./...
//
// Or on its own, where -fix applies the suggested fixes:
//
//	catenalint -fix ./...
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/NerdMeNot/catena/lint"
)

func main() {
	multichecker.Main(lint.Analyzers()...)
}
