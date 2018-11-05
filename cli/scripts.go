package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"regexp"
	"strconv"

	"github.com/capitancambio/blackterm"
	"github.com/capitancambio/go-subcommand"
	"github.com/daisy/pipeline-clientlib-go"
)

//set the last id path (in utils)
var LastIdPath = getLastIdPath(runtime.GOOS)

//Represents the job request
type JobRequest struct {
	Script     string               //Script id to call
	Nicename   string               //Job's nicename
	Priority   string               //Job's priority
	Options    map[string][]string  //Options for the script
	Inputs     map[string][]url.URL //Input ports for the script
	Data       []byte               //Data to send with the job request
	Background bool                 //Send the request and return
}

//Creates a new JobRequest
func newJobRequest() *JobRequest {
	return &JobRequest{
		Options: make(map[string][]string),
		Inputs:  make(map[string][]url.URL),
	}
}

//Convinience method to add several scripts to a client
func (c *Cli) AddScripts(scripts []pipeline.Script, link *PipelineLink) error {
	for _, s := range scripts {
		if _, err := scriptToCommand(s, c, link); err != nil {
			return err
		}
	}
	return nil
}

//Executes a job request
type jobExecution struct {
	link       *PipelineLink
	req        *JobRequest
	output     string
	verbose    bool
	persistent bool
	zipped     bool
}

func (j jobExecution) run(stdOut io.Writer) error {
	log.Printf("run data len %v\n", len(j.req.Data))
	//manual check of output
	if !j.req.Background && j.output == "" {
		return errors.New("--output option is mandatory if the job is not running in the req.Background")
	}
	if j.req.Background && j.output != "" {
		fmt.Printf("Warning: --output option ignored as the job will run in the background\n")
	}
	storeId := j.req.Background || j.persistent
	//send the job
	job, messages, err := j.link.Execute(*(j.req))
	if err != nil {
		return err
	}
	fmt.Fprintf(stdOut, "Job %v sent to the server\n", job.Id)
	//store id if it suits
	if storeId {
		err = storeLastId(job.Id)
		if err != nil {
			return err
		}
	}
	//get realtime messages and status from the webservice
	status := job.Status
	for msg := range messages {
		if msg.Error != nil {
			err = msg.Error
			return err
		}
		//print messages
		if j.verbose {
			fmt.Fprintln(stdOut, msg.String())
		}
		status = msg.Status
	}

	if status != "ERROR" {
		//get the data
		if !j.req.Background {
			wc, err := zipProcessor(j.output, j.zipped)
			if err != nil {
				return err
			}
			if err := j.link.Results(job.Id, wc); err != nil {
				return err
			}
			if err := wc.Close(); err != nil {
				return err
			}

			if !j.persistent {
				_, err = j.link.Delete(job.Id)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdOut, "The job has been deleted from the server\n")
			}
			fmt.Fprintf(stdOut, "Job finished with status: %v\n", status)
		}

	}
	return nil
}

var commonFlags = []string{"--output", "--zip", "--nicename", "--priority", "--quiet", "--persistent", "--background"}

func getFlagName(name, prefix string, flags []subcommand.Flag) string {
	flaggedName := "--" + name

	allFlags := []string{}
	allFlags = append(allFlags, commonFlags...)
	allFlags = append(allFlags, flagsToString(flags)...)
	for _, f := range allFlags {
		if f == flaggedName {
			return prefix + name
		}

	}

	return name
}
func flagsToString(flags []subcommand.Flag) []string {
	res := make([]string, len(flags), len(flags))
	for idx, flag := range flags {
		res[idx] = flag.Long
	}
	return res
}

//Adds the command and flags to be able to call the script to the cli
func scriptToCommand(script pipeline.Script, cli *Cli, link *PipelineLink) (req *JobRequest, err error) {
	jobRequest := newJobRequest()
	jobRequest.Script = script.Id
	jobRequest.Background = false
	jExec := jobExecution{
		link:    link,
		req:     jobRequest,
		output:  "",
		verbose: true,
		zipped:  false,
	}
	command := cli.AddScriptCommand(script.Id, fmt.Sprintf("%s [v%s]", blackterm.MarkdownString(script.Description), script.Version), func(string, ...string) error {
		if err := jExec.run(cli.Output); err != nil {
			return err
		}
		return nil
	}, jobRequest)
	command.SetArity(0, "")

	for _, input := range script.Inputs {
		name := getFlagName(input.Name, "i-", command.Flags())
		shortDesc := input.ShortDesc
		longDesc := input.LongDesc
		if (shortDesc == "") {
			shortDesc = input.NiceName
		} else if len(longDesc) > len(shortDesc) {
			shortDesc += " [...]"
		}
		shortDesc = blackterm.MarkdownString(shortDesc)
		// FIXME: assumes markdown without html
		longDesc = blackterm.MarkdownString(longDesc)
		command.AddOption(name, "", shortDesc, longDesc, inputFunc(jobRequest, link)).Must(true)
	}

	for _, option := range script.Options {
		//desc:=option.Desc+
		name := getFlagName(option.Name, "x-", command.Flags())
		shortDesc := option.ShortDesc
		longDesc := option.LongDesc
		if (shortDesc == "") {
			shortDesc = option.NiceName
		} else if len(longDesc) > len(shortDesc) {
			shortDesc += " [...]"
		}
		shortDesc = blackterm.MarkdownString(shortDesc)
		// FIXME: assumes markdown without html
		longDesc = blackterm.MarkdownString(longDesc)
		command.AddOption(
			name, "", shortDesc, longDesc, optionFunc(jobRequest, link, option.Type, option.Sequence)).Must(option.Required)
	}
	command.AddOption("output", "o", "Path where to store the results. This option is mandatory when the job is not executed in the background", "", func(name, folder string) error {
		jExec.output = folder
		return nil
	})
	command.AddSwitch("zip", "z", "Write the output to a zip file rather than to a folder", func(string, string) error {
		jExec.zipped = true
		return nil
	})

	command.AddOption("nicename", "n", "Set job's nice name", "", func(name, nice string) error {
		jExec.req.Nicename = nice

		return nil
	})
	command.AddOption("priority", "r", "Set job's priority (high|medium|low)", "", func(name, priority string) error {
		if checkPriority(priority) {
			jExec.req.Priority = priority
			return nil
		} else {
			return fmt.Errorf("%s is not a valid priority. Allowed values are high, medium and low",
				priority)
		}
	})
	command.AddSwitch("quiet", "q", "Do not print the job's messages", func(string, string) error {
		jExec.verbose = false
		return nil
	})
	command.AddSwitch("persistent", "p", "Delete the job after it is executed", func(string, string) error {
		jExec.persistent = true
		return nil
	})

	command.AddSwitch("background", "b", "Sends the job and exits", func(string, string) error {
		jExec.req.Background = true
		return nil
	})

	return jobRequest, nil
}

func (c *ScriptCommand) addDataOption() {
	c.AddOption("data", "d", "Zip file containing the files to convert", "", func(name, path string) error {
		file, err := os.Open(path)
		defer func() {
			err := file.Close()
			if err != nil {
				log.Printf("Error closing file %v :%v", path, err.Error())
			}
		}()
		if err != nil {
			return err
		}
		c.req.Data, err = ioutil.ReadAll(file)
		//FIXME: this breaks the tests, but focused in a different thing right now
		//if err != nil {
		//return err
		//}
		log.Printf("data len %v\n", len(c.req.Data))
		return nil
	}).Must(true)
}

//Returns a function that fills the request info with the subcommand option name
//and value
func inputFunc(req *JobRequest, link *PipelineLink) func(string, string) error {
	return func(name, value string) (err error) {
		//control prefix
		basePath := getBasePath(link.IsLocal())
		if strings.HasPrefix("i-", name) {
			name = name[2:]
		}
		// FIXME: check if input is a sequence
		for _, path := range strings.Split(value, ",") {
			var u *url.URL
			u, err = pathToUri(path, basePath)
			if err != nil {
				return
			}
			req.Inputs[name] = append(req.Inputs[name], *u)
		}
		return
	}
}

//Returns a function that fills the request option with the subcommand option name
//and value
func optionFunc(req *JobRequest, link *PipelineLink, optionType pipeline.DataType, sequence bool) func(string, string) error {
	return func(name, value string) error {
		if strings.HasPrefix("x-", name) {
			name = name[2:]
		}
		var err error
		if sequence {
			for _, v := range strings.Split(value, ",") {
				v, err = validateOption(v, optionType, link)
				if err != nil {
					return validationError(name, v, err)
				}
				req.Options[name] = append(req.Options[name], v)
			}
		} else {
			value, err = validateOption(value, optionType, link)
			if err != nil {
				return validationError(name, value, err)
			}
			req.Options[name] = append(req.Options[name], value)
		}
		return nil
	}
}

func validationError(optionName, value string, cause error) error {
	msg := "'" + value + "' is not allowed as the value for option --" + optionName
	if cause != nil {
		msg += (": " + cause.Error())
	}
	return errors.New(msg)
}

func validateOption(value string, optionType pipeline.DataType, link *PipelineLink) (result string, err error) {
	result = value
	switch t := optionType.(type) {
	case pipeline.XsBoolean:
		var b bool
		b, err = strconv.ParseBool(value)
		if err == nil {
			result = strconv.FormatBool(b)
		}
	case pipeline.XsInteger:
		_, err = strconv.ParseInt(value, 0, 0)
	case pipeline.XsAnyURI:
		// _, err = url.Parse(value)
	case pipeline.AnyFileURI:
		var u *url.URL
		u, err = pathToUri(value, getBasePath(link.IsLocal()))
		if err == nil {
			result = u.String()
		}
	case pipeline.AnyDirURI:
		var u *url.URL
		u, err = pathToUri(value, getBasePath(link.IsLocal()))
		if err == nil {
			result = u.String()
		}
	case pipeline.Pattern:
		var match bool
		match, err = regexp.MatchString("^(?:" + t.Pattern + ")$", value)
		if err == nil && ! match {
			err = errors.New("does not match /" + t.Pattern + "/")
			return
		}
	case pipeline.Choice:
		for _, v := range t.Values {
			result, err = validateOption(value, v, link)
			if err == nil {
				break
			}
		}
		if err != nil {
			err = errors.New("does not match " + uncolor(optionTypeToString(t, "", "")))
			return
		}
	case pipeline.Value:
		if t.Value != value {
			err = errors.New("does not match '" + t.Value + "'")
			return
		}
	default:
	}
	if err != nil {
		err = errors.New("does not match " + uncolor(optionTypeToString(optionType, "", ""))+ ": " + err.Error())
	}
	return
}

//Gets the basepath. If the fwk accepts local uri's (file:///)
//getBasePath os.Getwd() otherwise it returns an empty string
func getBasePath(isLocal bool) string {
	if isLocal {
		base, err := os.Getwd()
		if err != nil {
			panic("Error while getting current directory:" + err.Error())
		}
		return base + "/"
	} else {
		return ""
	}
}

//Accepts several paths separated by separator and constructs the URLs
//relative to base path
func pathToUri(path string, basePath string) (u *url.URL, err error) {
	if basePath != "" {
		// localfs
		var baseUrl *url.URL
		if basePath != "" {
			if string(basePath[0]) != "/" {
				//for windows path to build a proper url
				basePath = "/" + basePath
			}
			baseUrl, err = url.Parse("file:" + basePath)
		}
		if err != nil {
			return
		}
		u, err = url.Parse(filepath.ToSlash(path))
		if err != nil {
			return
		}
		u = baseUrl.ResolveReference(u)
		// check that file exists (do it at the end so that the url resolving part can be tested)
		// FIXME: use basePath instead of implicit pwd
		// FIXME: does this also work for directories?
		_, err = os.Stat(path)
		if os.IsNotExist(err) {
			return
		} else {
			err = nil
		}
		if u.Scheme != "file" || ! u.IsAbs() {
			err = errors.New("unexpected error")
		}
	} else {
		// FIXME: check if file present in zip
		//TODO is opaque really apropriate?
		u = &url.URL{
			Opaque: filepath.ToSlash(path),
		}
	}
	return
}

func storeLastId(id string) error {
	file, err := os.Create(LastIdPath)
	if err != nil {
		return err
	}
	defer func() {
		file.Close()
	}()
	if _, err := file.Write([]byte(id)); err != nil {
		return err
	}
	return nil
}

func getLastId() (id string, err error) {
	idBuf, err := ioutil.ReadFile(LastIdPath)
	if err != nil {
		return "", err
	}
	return string(idBuf), nil
}
