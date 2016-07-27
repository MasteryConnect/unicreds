package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/apex/log/handlers/json"

	"github.com/MasteryConnect/unicreds"
	"github.com/alecthomas/kingpin"
)

var (
	app     = kingpin.New("unicreds", "A credential/secret storage command line tool.")
	csv     = app.Flag("csv", "Enable csv output for table data.").Short('c').Bool()
	debug   = app.Flag("debug", "Enable debug mode.").Short('d').Bool()
	logJSON = app.Flag("json", "Output results in JSON").Short('j').Bool()

	region = app.Flag("region", "Configure the AWS region").Short('r').String()

	dynamoTable = app.Flag("table", "DynamoDB table.").Default("credential-store").Short('t').String()
	alias       = app.Flag("alias", "KMS key alias.").Default("alias/credstash").Short('k').String()

	// commands
	cmdSetup = app.Command("setup", "Setup the dynamodb table used to store credentials.")

	cmdGet        = app.Command("get", "Get a credential from the store.")
	cmdGetName    = cmdGet.Arg("credential", "The name of the credential to get.").Required().String()
	cmdGetVersion = cmdGet.Arg("version", "The version of the credential to get.").String()

	cmdGetAll         = app.Command("getall", "Get latest credentials from the store.")
	cmdGetAllVersions = cmdGetAll.Flag("all", "List all versions").Bool()

	cmdList            = app.Command("list", "List latest credentials with names and version.")
	cmdListAllVersions = cmdList.Flag("all", "List all versions").Bool()

	cmdPut        = app.Command("put", "Put a credential into the store.")
	cmdPutName    = cmdPut.Arg("credential", "The name of the credential to store.").Required().String()
	cmdPutSecret  = cmdPut.Arg("value", "The value of the credential to store.").Required().String()
	cmdPutVersion = cmdPut.Arg("version", "Version to store with the credential.").Int()

	cmdPutFile           = app.Command("put-file", "Put a credential from a file into the store.")
	cmdPutFileName       = cmdPutFile.Arg("credential", "The name of the credential to store.").Required().String()
	cmdPutFileSecretPath = cmdPutFile.Arg("value", "Path to file containing the credential to store.").Required().String()
	cmdPutFileVersion    = cmdPutFile.Arg("version", "Version to store with the credential.").Int()

	cmdDelete     = app.Command("delete", "Delete a credential from the store.")
	cmdDeleteName = cmdDelete.Arg("credential", "The name of the credential to delete.").Required().String()

	// Version app version
	Version = "1.0.0"
)

func main() {
	app.Version(Version)

	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	if *logJSON {
		log.SetHandler(json.New(os.Stderr))
	} else {
		log.SetHandler(cli.Default)
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	unicreds.SetRegion(region)

	switch command {
	case cmdSetup.FullCommand():
		err := unicreds.Setup(dynamoTable)
		if err != nil {
			printFatalError(err)
		}
		log.WithFields(log.Fields{"status": "success"}).Info("Created table")
	case cmdGet.FullCommand():
		cred, err := unicreds.GetSecret(dynamoTable, *cmdGetName, *cmdGetVersion)
		if err != nil {
			printFatalError(err)
		}

		if *logJSON {
			log.WithFields(log.Fields{"name": *cmdGetName, "secret": cred.Secret, "status": "success"}).Info(cred.Secret)
		} else {
			// Or just print, out of backwards compatibility
			fmt.Println(cred.Secret)
		}

	case cmdPut.FullCommand():
		version, err := unicreds.ResolveVersion(dynamoTable, *cmdPutName, *cmdPutVersion)
		if err != nil {
			printFatalError(err)
		}

		err = unicreds.PutSecret(dynamoTable, *alias, *cmdPutName, *cmdPutSecret, version)
		if err != nil {
			printFatalError(err)
		}
		log.WithFields(log.Fields{"name": *cmdPutName, "version": version}).Info("stored")
	case cmdPutFile.FullCommand():
		version, err := unicreds.ResolveVersion(dynamoTable, *cmdPutFileName, *cmdPutFileVersion)
		if err != nil {
			printFatalError(err)
		}

		data, err := ioutil.ReadFile(*cmdPutFileSecretPath)
		if err != nil {
			printFatalError(err)
		}

		err = unicreds.PutSecret(dynamoTable, *alias, *cmdPutFileName, string(data), version)
		if err != nil {
			printFatalError(err)
		}
		log.WithFields(log.Fields{"name": *cmdPutName, "version": version}).Info("stored")
	case cmdList.FullCommand():
		creds, err := unicreds.ListSecrets(dynamoTable, *cmdListAllVersions)
		if err != nil {
			printFatalError(err)
		}

		table := unicreds.NewTable(os.Stdout)
		table.SetHeaders([]string{"Name", "Version", "Created-At"})

		if *csv {
			table.SetFormat(unicreds.TableFormatCSV)
		}

		for _, cred := range creds {
			table.Write([]string{cred.Name, cred.Version, cred.CreatedAtDate()})
		}
		if err = table.Render(); err != nil {
			printFatalError(err)
		}
	case cmdGetAll.FullCommand():
		creds, err := unicreds.GetAllSecrets(dynamoTable, *cmdGetAllVersions)
		if err != nil {
			printFatalError(err)
		}

		table := unicreds.NewTable(os.Stdout)
		table.SetHeaders([]string{"Name", "Secret"})

		if *csv {
			table.SetFormat(unicreds.TableFormatCSV)
		}

		for _, cred := range creds {
			table.Write([]string{cred.Name, cred.Secret})
		}

		if err = table.Render(); err != nil {
			printFatalError(err)
		}
	case cmdDelete.FullCommand():
		err := unicreds.DeleteSecret(dynamoTable, *cmdDeleteName)
		if err != nil {
			printFatalError(err)
		}
	}
}

func printFatalError(err error) {
	log.WithError(err).Error("failed")
	os.Exit(1)
}
