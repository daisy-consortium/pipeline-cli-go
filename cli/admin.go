package cli

//TODO get rid of link methods that just bypass the to the pipeline
import (
	//"github.com/capitancambio/go-subcommand"
	"fmt"
	"os"
	"text/template"

	"github.com/daisy-consortium/pipeline-clientlib-go"
)

const (
	TmplClients = `client_id         (role)

{{range .}}{{.Id}}          ({{.Role}})
{{end}}
`
	TmplClient = `
Client id:      {{.Id}}
Role:           {{.Role}}
Contact:        {{.Contact}}
Secret:         ****

`
	TmplProperties = ` Name          Value           Bundle
        
{{range .}}{{.Name}}            {{.Value}}              {{.BundleName}}
{{end}}
`
	TmplSizes = `JobId                 		Context Size    Output Size    Log Size    Total Size

{{range .}}{{.Id}}   {{format .Context}}    {{format .Output}}    {{format .Log}}    {{ total . | format}}
{{end}}

`
)

func (c *Cli) AddClientListCommand(link PipelineLink) {
	c.AddCommand("list", "Returns the list of the available clients", func(command string, args ...string) error {
		clients, err := link.pipeline.Clients()
		if err != nil {
			return err
		}
		tmpl, err := template.New("list").Parse(TmplClients)
		if err != nil {
			return err
		}
		err = tmpl.Execute(os.Stdout, clients)
		return nil
	})
}

func (c *Cli) AddNewClientCommand(link PipelineLink) {
	client := &pipeline.Client{}
	cmd := c.AddCommand("create", "Creates a new client", func(command string, args ...string) error {
		res, err := link.pipeline.NewClient(*client)
		if err != nil {
			return err
		}
		tmpl, err := template.New("client").Parse(TmplClient)
		if err != nil {
			return err
		}
		fmt.Println("Client successfully created")
		err = tmpl.Execute(os.Stdout, res)
		return nil
	})
	cmd.AddOption("id", "i", "Client id (must be unique)", func(string, value string) error {
		client.Id = value
		return nil
	}).Must(true)

	cmd.AddOption("secret", "s", "Client secret", func(string, value string) error {
		client.Secret = value
		return nil
	}).Must(true)

	cmd.AddOption("role", "r", "Client role  (ADMIN,CLIENTAPP)", func(string, value string) error {
		if value != "ADMIN" && value != "CLIENTAPP" {
			return fmt.Errorf("%v is not a valid role", value)
		}
		client.Role = value
		return nil
	}).Must(true)

	cmd.AddOption("contact", "c", "Client e-mail address ", func(string, value string) error {
		client.Contact = value
		return nil
	})

}

func (c *Cli) AddDeleteClientCommand(link PipelineLink) {
	c.AddCommand("delete", "Deletes a client", func(command string, args ...string) error {
		id := args[0]
		_, err := link.pipeline.DeleteClient(id)
		if err != nil {
			return err
		}
		fmt.Printf("Client %v deleted\n", id)
		return nil
	}).SetArity(1, "CLIENT_ID")
}

func (c *Cli) AddClientCommand(link PipelineLink) {

	c.AddCommand("client", "Prints the detailed client inforamtion", func(command string, args ...string) error {
		id := args[0]
		client, err := link.pipeline.Client(id)
		if err != nil {
			return err
		}
		tmpl, err := template.New("client").Parse(TmplClient)
		if err != nil {
			return err
		}
		return tmpl.Execute(os.Stdout, client)
	}).SetArity(1, "CLIENT_ID")
}
func (c *Cli) AddModifyClientCommand(link PipelineLink) {
	client := &pipeline.Client{}
	cmd := c.AddCommand("modify", "Modifies a client", func(command string, args ...string) error {
		id := args[0]
		client.Id = id
		old, err := link.pipeline.Client(id)
		if err != nil {
			return err
		}
		if len(client.Secret) == 0 {
			client.Secret = old.Secret
		}
		if len(client.Role) == 0 {
			client.Role = old.Role
		}
		if len(client.Contact) == 0 {
			client.Contact = old.Contact
		}
		res, err := link.pipeline.ModifyClient(*client, id)
		if err != nil {
			return err
		}
		tmpl, err := template.New("client").Parse(TmplClient)
		if err != nil {
			return err
		}
		fmt.Println("Client successfully modified")
		err = tmpl.Execute(os.Stdout, res)
		return nil
	}).SetArity(1, "CLIENT_ID")
	cmd.AddOption("secret", "s", "Client secret", func(string, value string) error {
		client.Secret = value
		return nil
	})

	cmd.AddOption("role", "r", "Client role  (ADMIN,CLIENTAPP)", func(string, value string) error {
		if value != "ADMIN" && value != "CLIENTAPP" {
			return fmt.Errorf("%v is not a valid role", value)
		}
		client.Role = value
		return nil
	})

	cmd.AddOption("contact", "c", "Client e-mail address ", func(string, value string) error {
		client.Contact = value
		return nil
	})

}

func (c *Cli) AddPropertyListCommand(link PipelineLink) {
	c.AddCommand("properties", "List the pipeline ws runtime properties ", func(command string, args ...string) error {
		properties, err := link.pipeline.Properties()
		if err != nil {
			return err
		}
		tmpl, err := template.New("props").Parse(TmplProperties)
		if err != nil {
			return err
		}
		err = tmpl.Execute(os.Stdout, properties)

		return nil
	})
}

func (c *Cli) AddSizesCommand(link PipelineLink) {
	list := false
	unitFormatter := func(size int) string {
		return fmt.Sprintf("%d", size)
	}
	cmd := c.AddCommand("sizes", "Prits the total size or a detailed list of job data stored in the server", func(command string, args ...string) error {
		sizes, err := link.Sizes()
		if err != nil {
			return err
		}
		if !list {
			fmt.Printf("Total %s\n", unitFormatter(sizes.Total))
		} else {
			funcMap := template.FuncMap{
				"format": unitFormatter,
				"total": func(size pipeline.JobSize) int {
					return size.Context + size.Output + size.Log
				},
			}
			tmpl, err := template.New("sizes").Funcs(funcMap).Parse(TmplSizes)
			if err != nil {
				return err
			}
			err = tmpl.Execute(os.Stdout, sizes.JobSizes)
		}

		return nil
	})
	cmd.AddSwitch("list", "l", "Displays a detailed list rather than the total size", func(string, string) error {
		list = true
		return nil
	})
	cmd.AddSwitch("human", "h", "Use a more human readable size (megabytes)", func(string, string) error {
		unitFormatter = func(size int) string {
			return fmt.Sprintf("%.4fM", float64(size)/(1048576))
		}
		return nil
	})

}
