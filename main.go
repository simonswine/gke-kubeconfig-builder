package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

func processKubeEnv(env io.Reader) (kubeconfig string, environment string, err error) {
	fields := make(map[string]string)

	buf := new(bytes.Buffer)

	scanner := bufio.NewScanner(env)
	for scanner.Scan() {
		line := scanner.Text()
		field := strings.SplitN(line, ": ", 2)
		fields[field[0]] = field[1]

		buf.WriteString(field[0])
		buf.WriteString("=")
		buf.WriteString(field[1])
		buf.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Config
users:
- name: kubelet
  user:
    client-certificate-data: %s
    client-key-data: %s
clusters:
- name: local
  cluster:
    certificate-authority-data: %s
    server: https://%s
contexts:
- context:
    cluster: local
    user: kubelet
  name: service-account-context
current-context: service-account-context
`, fields["KUBELET_CERT"], fields["KUBELET_KEY"], fields["CA_CERT"], fields["KUBERNETES_MASTER_NAME"]), buf.String(), nil

}

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	if len(os.Args) < 3 {
		logger.Fatal().Msgf("please specify google cloud project and cluster name as arguments: %s <project> <cluster-name>", os.Args[0])
	}

	project := os.Args[1]
	clusterName := os.Args[2]
	logger = logger.With().Str("project", project).Str("cluster-name", clusterName).Logger()

	logger.Info().Msg("getting kubeconfig")

	// Use oauth2.NoContext if there isn't a good context to pass in.
	ctx := context.Background()

	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		logger.Fatal().Err(err)
	}
	computeService, err := compute.New(client)
	if err != nil {
		logger.Fatal().Err(err)
	}

	list, err := computeService.InstanceTemplates.List(project).Do()
	if err != nil {
		logger.Fatal().Err(err)
	}

	// get kube-env from latest instance template for cluster
	var latestKubeEnv string
	var latestCreation time.Time
	for _, templateElement := range list.Items {
		createdTime, err := time.Parse(time.RFC3339, templateElement.CreationTimestamp)
		if err != nil {
			logger.Warn().Err(err)
			continue
		}

		if latestCreation.After(createdTime) {
			continue
		}

		var templateClusterName string
		var templateKubeEnv string

		for _, metadataElement := range templateElement.Properties.Metadata.Items {
			if metadataElement.Key == "cluster-name" {
				templateClusterName = *metadataElement.Value
			}
			if metadataElement.Key == "kube-env" {
				templateKubeEnv = *metadataElement.Value
			}
		}

		if clusterName != templateClusterName {
			continue
		}

		latestKubeEnv = templateKubeEnv
		latestCreation = createdTime
	}

	if latestKubeEnv == "" {
		logger.Fatal().Msg("no kube-env found")
	}

	kubeconfigData, environmentData, err := processKubeEnv(strings.NewReader(latestKubeEnv))
	if err != nil {
		logger.Fatal().Err(err)
	}

	environmentFile := "environment"
	err = ioutil.WriteFile(environmentFile, []byte(environmentData), 0600)
	if err != nil {
		logger.Fatal().Err(err)
	}
	logger.Info().Msgf("wrote %s", environmentFile)

	kubeconfigFile := "kubeconfig"

	err = ioutil.WriteFile(kubeconfigFile, []byte(kubeconfigData), 0600)
	if err != nil {
		logger.Fatal().Err(err)
	}
	logger.Info().Msgf("wrote %s", kubeconfigFile)

}
