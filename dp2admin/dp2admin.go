package main

import (
	"fmt"
	"log"
	"os"

	"github.com/daisy/pipeline-cli-go/cli"
)

var minJavaVersion = 1.7

func main() {
	log.SetFlags(log.Lshortfile)
	if err := cli.AssertJava(minJavaVersion); err != nil {
		fmt.Printf(
			"Java version error:\n\tPlease make sure that java is accessible and the version is equal or greater than %v\n\tError: %s\n",
			minJavaVersion,
			err.Error(),
		)
		os.Exit(-1)
	}
	cnf := cli.NewConfig()
	// proper error handlign missing

	link := cli.NewLink(cnf)

	comm, err := cli.NewCli("dp2admin", link)
	if err != nil {
		fmt.Printf("Error creating client:\n\t%v\n", err)
		os.Exit(-1)
	}
	comm.WithScripts = false

	cli.AddHaltCommand(comm, *link)
	comm.AddClientListCommand(*link)
	comm.AddNewClientCommand(*link)
	comm.AddDeleteClientCommand(*link)
	comm.AddModifyClientCommand(*link)
	comm.AddClientCommand(*link)
	comm.AddPropertyListCommand(*link)
	comm.AddSizesCommand(*link)

	err = comm.Run(os.Args[1:])
	if err != nil {
		fmt.Printf("Error:\n\t%v\n", err)
		os.Exit(-1)
	}
}
