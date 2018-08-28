package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	providerTypes "github.com/openfaas/faas-provider/types"
	"github.com/openfaas/faas/gateway/metrics"
	"github.com/openfaas/faas/gateway/requests"
)

var dryRun bool

type Credentials struct {
	Username string
	Password string
}

func main() {

	flag.BoolVar(&dryRun, "dry-run", false, "use dry-run for scaling events")
	flag.Parse()

	credentials := Credentials{}
	gatewayURL := os.Getenv("gateway_url")
	prometheusHost := os.Getenv("prometheus_host")

	if len(gatewayURL) == 0 {
		log.Panic("env-var gateway_url must be set")
	}

	if len(prometheusHost) == 0 {
		log.Panic("env-var prometheus_host must be set")
	}

	val, err := readFile("/run/secrets/basic-auth-user")
	if err == nil {
		credentials.Username = val
	} else {
		log.Printf("Unable to read username: %s", err)
	}

	passwordVal, passErr := readFile("/run/secrets/basic-auth-password")
	if passErr == nil {
		credentials.Password = passwordVal
	} else {
		log.Printf("Unable to read password: %s", err)
	}

	inactivityDuration := time.Minute * 5
	if val, exists := os.LookupEnv("inactivity_duration"); exists {
		parsedVal, parseErr := time.ParseDuration(val)
		if parseErr != nil {
			log.Printf("error parsing inactivity_duration: %s\n", parseErr.Error())
		}
		inactivityDuration = parsedVal
	}

	prometheusPort := 9090
	if val, exists := os.LookupEnv("prometheus_port"); exists {
		port, parseErr := strconv.Atoi(val)
		if parseErr != nil {
			log.Panicln(parseErr)
		}
		prometheusPort = port
	}

	reconcileInterval := time.Second * 30
	if val, exists := os.LookupEnv("reconcile_interval"); exists {
		parsedVal, parseErr := time.ParseDuration(val)
		if parseErr != nil {
			log.Printf("error parsing reconcile_interval: %s\n", parseErr.Error())
		}
		reconcileInterval = parsedVal
	}

	fmt.Printf(`dry_run: %t
gateway_url: %s
inactivity_duration: %s `, dryRun, gatewayURL, inactivityDuration)

	if len(gatewayURL) == 0 {
		fmt.Println("gateway_url (faas-netes/faas-swarm) is required.")
		os.Exit(1)
	}

	client := &http.Client{}
	for {

		reconcile(client, gatewayURL, prometheusHost, prometheusPort, inactivityDuration, &credentials)
		time.Sleep(reconcileInterval)
		fmt.Printf("\n")
	}
}

func readFile(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		data, readErr := ioutil.ReadFile(path)
		return strings.TrimSpace(string(data)), readErr
	}
	return "", nil
}

func buildMetricsMap(client *http.Client, functions []requests.Function, prometheusHost string, prometheusPort int, inactivityDuration time.Duration) map[string]float64 {
	query := metrics.NewPrometheusQuery(prometheusHost, prometheusPort, client)
	metrics := make(map[string]float64)

	duration := fmt.Sprintf("%dm", int(inactivityDuration.Minutes()))
	// duration := "5m"

	for _, function := range functions {
		querySt := url.QueryEscape(`sum(rate(gateway_function_invocation_total{function_name="` + function.Name + `", code=~".*"}[` + duration + `])) by (code, function_name)`)
		// fmt.Println(function.Name)
		res, err := query.Fetch(querySt)
		if err != nil {
			log.Println(err)
			continue
		}

		if len(res.Data.Result) > 0 {
			for _, v := range res.Data.Result {
				fmt.Println(v)
				if v.Metric.FunctionName == function.Name {
					metricValue := v.Value[1]
					switch metricValue.(type) {
					case string:

						f, strconvErr := strconv.ParseFloat(metricValue.(string), 64)
						if strconvErr != nil {
							log.Printf("Unable to convert value for metric: %s\n", strconvErr)
							continue
						}

						if _, exists := metrics[function.Name]; !exists {
							metrics[function.Name] = 0
						}

						metrics[function.Name] = metrics[function.Name] + f
					}
				}
			}

		}

	}

	return metrics
}

func reconcile(client *http.Client, gatewayURL, prometheusHost string, prometheusPort int, inactivityDuration time.Duration, credentials *Credentials) {
	functions, err := queryFunctions(client, gatewayURL, credentials)

	if err != nil {
		log.Println(err)
		return
	}

	metrics := buildMetricsMap(client, functions, prometheusHost, prometheusPort, inactivityDuration)

	for _, fn := range functions {
		if v, found := metrics[fn.Name]; found {
			if v == float64(0) {
				fmt.Printf("%s\tidle\n", fn.Name)

				if val, _ := getReplicas(client, gatewayURL, fn.Name, credentials); val != nil && val.AvailableReplicas > 0 {
					sendScaleEvent(client, gatewayURL, fn.Name, uint64(0), credentials)
				}

			} else {
				fmt.Printf("%s\tactive: %f\n", fn.Name, v)
			}
		}
	}
}

func getReplicas(client *http.Client, gatewayURL string, name string, credentials *Credentials) (*requests.Function, error) {
	item := &requests.Function{}
	var err error

	req, _ := http.NewRequest(http.MethodGet, gatewayURL+"system/function/"+name, nil)
	req.SetBasicAuth(credentials.Username, credentials.Password)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	bytesOut, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(bytesOut, &item)

	return item, err
}

func queryFunctions(client *http.Client, gatewayURL string, credentials *Credentials) ([]requests.Function, error) {
	list := []requests.Function{}
	var err error

	req, _ := http.NewRequest(http.MethodGet, gatewayURL+"system/functions", nil)
	req.SetBasicAuth(credentials.Username, credentials.Password)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	bytesOut, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(bytesOut, &list)

	return list, err
}

func sendScaleEvent(client *http.Client, gatewayURL string, name string, replicas uint64, credentials *Credentials) {
	if dryRun {
		fmt.Printf("dry-run: Scaling %s to %d replicas\n", name, replicas)
		return
	}

	scaleReq := providerTypes.ScaleServiceRequest{
		ServiceName: name,
		Replicas:    replicas,
	}

	var err error

	bodyBytes, _ := json.Marshal(scaleReq)
	bodyReader := bytes.NewReader(bodyBytes)

	req, _ := http.NewRequest(http.MethodPost, gatewayURL+"system/scale-function/"+name, bodyReader)
	req.SetBasicAuth(credentials.Username, credentials.Password)

	res, err := client.Do(req)

	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Scale", name, res.StatusCode, replicas)

	if res.Body != nil {
		defer res.Body.Close()
	}
}
