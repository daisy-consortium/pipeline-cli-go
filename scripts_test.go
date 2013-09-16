package main

import (
	"fmt"
	//"github.com/capitancambio/go-subcommand"
	//"github.com/daisy-consortium/pipeline-clientlib-go"
	"io/ioutil"
	"log"
	"testing"
)



func TestGetBasePath(t *testing.T) {
	//return os.Getwd()
	basePath := getBasePath(true)
	if len(basePath) == 0 {
		t.Error("Base path is 0")
	}
	if basePath[len(basePath)-1] != "/"[0] {
		t.Error("Last element of the basePath should be /")
	}
	basePath = getBasePath(false)
	if len(basePath) != 0 {
		t.Errorf("Base path len is !=0: %v", basePath)
	}
}



func TestParseInputs(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	inputs := fmt.Sprintf("%v,%v", in1, in2)
	urls, err := pathToUri(inputs, ",", "")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if urls[0].String() != in1 {
		t.Errorf("Url 1 is not formatted %v", urls[0].String())
	}

	if urls[1].String() != in2 {
		t.Errorf("Url 2 is not formatted %v", urls[1].String())
	}
}


func TestParseInputsBased(t *testing.T) {
	inputs := fmt.Sprintf("%v,%v", in1, in2)
	urls, err := pathToUri(inputs, ",", "/mydata/")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	//println(urls[0].String())
	if urls[0].String() != "file:///mydata/"+"tmp/dir1/file.xml" {
		t.Errorf("Url 1 is not formated %v", urls[0].String())
	}

	if urls[1].String() != "file:///mydata/"+"tmp/dir2/file.xml" {
		t.Errorf("Url 1 is not formated %v", urls[1].String())
	}
}


func TestScriptToCommand(t *testing.T) {
	link := PipelineLink{pipeline: &PipelineTest{true, 0}}
	cli,err := NewCli("test",link)
	if err != nil {
		t.Error("Unexpected error")
	}
	jobRequest, err := scriptToCommand(SCRIPT,cli,link,false)
	if err != nil {
		t.Error("Unexpected error")
	}
	//parser.Parse([]string{"test","--i-source","value"})
	_, err = cli.Parse([]string{"test", "--i-source", "./tmp/file", "--i-single", "./tmp/file2", "--x-test-opt", "./myfile.xml", "--x-another-opt", "true"})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if jobRequest.Script != "test" {
		t.Error("script not set")
	}
	if jobRequest.Inputs["source"][0].String() != "./tmp/file" {
		t.Errorf("Input source not set %v", jobRequest.Inputs["source"][0].String())
	}
	if jobRequest.Inputs["single"][0].String() != "./tmp/file2" {
		t.Errorf("Input source not set %v", jobRequest.Inputs["source"][0].String())
	}
	if jobRequest.Options["test-opt"][0] != "./myfile.xml" {
		t.Errorf("Option test opt not set %v", jobRequest.Options["test-opt"][0])
	}

	if jobRequest.Options["another-opt"][0] != "true" {
		t.Errorf("Option test opt not set %v", jobRequest.Options["another-opt"][0])
	}
}
func TestCliRequiredOptions(t *testing.T) {
	link := PipelineLink{pipeline: &PipelineTest{true, 0}}
	cli,err := NewCli("test",link)
	if err != nil {
		t.Error("Unexpected error")
	}
	_, err = scriptToCommand(SCRIPT,cli,link,false)
	if err != nil {
		t.Error("Unexpected error")
	}
	//parser.Parse([]string{"test","--i-source","value"})
	err = cli.Run([]string{"test", "--i-source", "./tmp/file", "--i-single", "./tmp/file2", "--x-another-opt", "true"})
	if err == nil {
		t.Errorf("Missing required option wasn't thrown")
	}
	err = cli.Run([]string{"./tmp/file", "--i-single", "./tmp/file2", "--x-another-opt", "true"})
	if err == nil {
		t.Errorf("Missing required input wasn't thrown")
	}
}



