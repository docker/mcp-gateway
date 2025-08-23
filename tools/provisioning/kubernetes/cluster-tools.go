package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// MCP Gateway Kubernetes provisioning and management utility
// Provides commands for cleanup, ConfigMap/Secret provisioning, and resource management
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  session <session-id> [namespace]     - Clean up pods for specific session\n")
		fmt.Fprintf(os.Stderr, "  stale <max-age> [namespace]          - Clean up pods older than max-age (e.g., '24h', '2h30m')\n")
		fmt.Fprintf(os.Stderr, "  list [namespace]                     - List all MCP gateway managed pods\n")
		fmt.Fprintf(os.Stderr, "  create-configmap <name> <data.json> [namespace] - Create ConfigMap for cluster provider\n")
		fmt.Fprintf(os.Stderr, "  create-secret <name> <data.json> [namespace]    - Create Secret for cluster provider\n")
		fmt.Fprintf(os.Stderr, "  show-example <server-name>           - Show example JSON for ConfigMap/Secret creation\n")
		fmt.Fprintf(os.Stderr, "  extract-data <server-name>           - Extract real ConfigMap/Secret data from catalog\n")
		fmt.Fprintf(os.Stderr, "  generate-env <server-names> <base-name> - Generate separate .env files for configs and secrets (comma-delimited servers)\n")
		fmt.Fprintf(os.Stderr, "  populate-configmap <config-env-file> <configmap-name> [namespace] - Create/update ConfigMap from config .env file\n")
		fmt.Fprintf(os.Stderr, "  populate-secret <secret-env-file> <secret-name> [namespace] - Create/update Secret from secret .env file\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s session mcp-gateway-abc12345\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s stale 24h default\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s list\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s create-configmap my-config '{\"ENABLE_ADDING_ACTORS\":\"false\"}' default\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s create-secret my-secrets '{\"APIFY_TOKEN\":\"your-token\"}' default\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s show-example firewalla-mcp-server\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s extract-data firewalla-mcp-server\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s generate-env firewalla-mcp-server firewalla\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s generate-env firewalla-mcp-server,apify-mcp-server multi\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s populate-configmap multi-config.env my-mcp-config default\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s populate-secret multi-secret.env my-mcp-secrets default\n", os.Args[0])
		os.Exit(1)
	}

	// Get Kubernetes client
	clientset, err := getKubernetesClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to Kubernetes: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure kubectl is configured and you have access to the cluster.\n")
		os.Exit(1)
	}

	ctx := context.Background()
	command := os.Args[1]

	switch command {
	case "session":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: session command requires session ID\n")
			os.Exit(1)
		}
		sessionID := os.Args[2]
		namespace := "default"
		if len(os.Args) > 3 {
			namespace = os.Args[3]
		}
		err = cleanupBySession(ctx, clientset, sessionID, namespace)

	case "stale":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: stale command requires max age (e.g., '24h', '2h30m')\n")
			os.Exit(1)
		}
		maxAge, err := time.ParseDuration(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing duration '%s': %v\n", os.Args[2], err)
			os.Exit(1)
		}
		namespace := "default"
		if len(os.Args) > 3 {
			namespace = os.Args[3]
		}
		err = cleanupStale(ctx, clientset, maxAge, namespace)

	case "list":
		namespace := "default"
		if len(os.Args) > 2 {
			namespace = os.Args[2]
		}
		err = listMCPPods(ctx, clientset, namespace)

	case "create-configmap":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: create-configmap command requires name and data\n")
			os.Exit(1)
		}
		name := os.Args[2]
		dataJSON := os.Args[3]
		namespace := "default"
		if len(os.Args) > 4 {
			namespace = os.Args[4]
		}
		err = createConfigMap(ctx, clientset, name, dataJSON, namespace)

	case "create-secret":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: create-secret command requires name and data\n")
			os.Exit(1)
		}
		name := os.Args[2]
		dataJSON := os.Args[3]
		namespace := "default"
		if len(os.Args) > 4 {
			namespace = os.Args[4]
		}
		err = createSecret(ctx, clientset, name, dataJSON, namespace)

	case "show-example":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: show-example command requires server name\n")
			os.Exit(1)
		}
		serverName := os.Args[2]
		err = showExampleData(serverName)

	case "extract-data":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: extract-data command requires server name\n")
			os.Exit(1)
		}
		serverName := os.Args[2]
		err = extractRealData(serverName)

	case "generate-env":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: generate-env command requires server names and base name\n")
			os.Exit(1)
		}
		serverNames := os.Args[2]
		baseName := os.Args[3]
		err = generateSeparateEnvFiles(serverNames, baseName)

	case "populate-configmap":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: populate-configmap command requires config env file and configmap name\n")
			os.Exit(1)
		}
		envFile := os.Args[2]
		configName := os.Args[3]
		namespace := "default"
		if len(os.Args) > 4 {
			namespace = os.Args[4]
		}
		err = populateConfigFromEnvFile(ctx, clientset, envFile, configName, namespace)

	case "populate-secret":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: populate-secret command requires secret env file and secret name\n")
			os.Exit(1)
		}
		envFile := os.Args[2]
		secretName := os.Args[3]
		namespace := "default"
		if len(os.Args) > 4 {
			namespace = os.Args[4]
		}
		err = populateSecretFromEnvFile(ctx, clientset, envFile, secretName, namespace)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", command)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// getKubernetesClient returns a Kubernetes clientset using kubeconfig
func getKubernetesClient() (kubernetes.Interface, error) {
	// Try in-cluster config first
	if config, err := rest.InClusterConfig(); err == nil {
		return kubernetes.NewForConfig(config)
	}

	// Fall back to kubeconfig
	kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		kubeconfigPath = kubeconfigEnv
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

// cleanupBySession removes all pods with the specified session ID
func cleanupBySession(ctx context.Context, clientset kubernetes.Interface, sessionID, namespace string) error {
	fmt.Printf("Cleaning up pods for session '%s' in namespace '%s'...\n", sessionID, namespace)

	labelSelector := fmt.Sprintf("mcp-gateway.docker.com/session=%s", sessionID)
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		fmt.Printf("No pods found for session '%s'\n", sessionID)
		return nil
	}

	fmt.Printf("Found %d pods to delete:\n", len(pods.Items))
	deletePolicy := metav1.DeletePropagationForeground

	for _, pod := range pods.Items {
		fmt.Printf("  Deleting pod: %s\n", pod.Name)
		err := clientset.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		if err != nil {
			fmt.Printf("    Warning: Failed to delete pod %s: %v\n", pod.Name, err)
		}
	}

	fmt.Printf("Session cleanup completed\n")
	return nil
}

// cleanupStale removes pods older than the specified duration
func cleanupStale(ctx context.Context, clientset kubernetes.Interface, maxAge time.Duration, namespace string) error {
	fmt.Printf("Cleaning up stale pods older than %v in namespace '%s'...\n", maxAge, namespace)

	labelSelector := "app.kubernetes.io/managed-by=mcp-gateway"
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		fmt.Printf("No MCP gateway managed pods found\n")
		return nil
	}

	cutoffTime := time.Now().Add(-maxAge)
	var stalePods []string

	for _, pod := range pods.Items {
		if pod.CreationTimestamp.Time.Before(cutoffTime) {
			stalePods = append(stalePods, pod.Name)
		}
	}

	if len(stalePods) == 0 {
		fmt.Printf("No stale pods found (all pods are newer than %v)\n", maxAge)
		return nil
	}

	fmt.Printf("Found %d stale pods to delete:\n", len(stalePods))
	deletePolicy := metav1.DeletePropagationForeground

	for _, podName := range stalePods {
		fmt.Printf("  Deleting stale pod: %s\n", podName)
		err := clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		if err != nil {
			fmt.Printf("    Warning: Failed to delete pod %s: %v\n", podName, err)
		}
	}

	fmt.Printf("Stale pod cleanup completed\n")
	return nil
}

// listMCPPods lists all pods managed by mcp-gateway
func listMCPPods(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	fmt.Printf("MCP Gateway managed pods in namespace '%s':\n\n", namespace)

	labelSelector := "app.kubernetes.io/managed-by=mcp-gateway"
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		fmt.Printf("No MCP gateway managed pods found\n")
		return nil
	}

	fmt.Printf("%-30s %-20s %-15s %-12s %s\n", "NAME", "SESSION", "STATUS", "AGE", "SERVER")
	fmt.Printf("%-30s %-20s %-15s %-12s %s\n", "----", "-------", "------", "---", "------")

	for _, pod := range pods.Items {
		sessionID := pod.Labels["mcp-gateway.docker.com/session"]
		if sessionID == "" {
			sessionID = "<unknown>"
		}

		serverName := pod.Labels["app.kubernetes.io/name"]
		if serverName == "" {
			serverName = "<unknown>"
		}

		age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
		status := string(pod.Status.Phase)

		fmt.Printf("%-30s %-20s %-15s %-12s %s\n",
			pod.Name, sessionID, status, age.String(), serverName)
	}

	fmt.Printf("\nTotal: %d pods\n", len(pods.Items))
	return nil
}

// createConfigMap creates a Kubernetes ConfigMap with the provided data
func createConfigMap(ctx context.Context, clientset kubernetes.Interface, name, dataJSON, namespace string) error {
	fmt.Printf("Creating ConfigMap '%s' in namespace '%s'...\n", name, namespace)

	// Parse JSON data
	var data map[string]string
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Create ConfigMap object
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":   "mcp-gateway-provisioning",
				"app.kubernetes.io/component":    "config",
				"mcp-gateway.docker.com/purpose": "cluster-provider",
			},
		},
		Data: data,
	}

	// Create the ConfigMap
	_, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	fmt.Printf("✅ ConfigMap '%s' created successfully\n", name)
	fmt.Printf("   Data keys: %v\n", getKeys(data))
	return nil
}

// createSecret creates a Kubernetes Secret with the provided data
func createSecret(ctx context.Context, clientset kubernetes.Interface, name, dataJSON, namespace string) error {
	fmt.Printf("Creating Secret '%s' in namespace '%s'...\n", name, namespace)

	// Parse JSON data
	var stringData map[string]string
	if err := json.Unmarshal([]byte(dataJSON), &stringData); err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Create Secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":   "mcp-gateway-provisioning",
				"app.kubernetes.io/component":    "secret",
				"mcp-gateway.docker.com/purpose": "cluster-provider",
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: stringData,
	}

	// Create the Secret
	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}

	fmt.Printf("✅ Secret '%s' created successfully\n", name)
	fmt.Printf("   Data keys: %v\n", getKeys(stringData))
	return nil
}

// showExampleData shows example JSON for a specific server's ConfigMap and Secret data
func showExampleData(serverName string) error {
	fmt.Printf("Example data for server '%s':\n\n", serverName)

	// Define examples for known servers
	switch serverName {
	case "firewalla-mcp-server":
		fmt.Println("ConfigMap data (static environment variables):")
		fmt.Println(`{`)
		fmt.Println(`  "MCP_WAVE0_ENABLED": "true",`)
		fmt.Println(`  "MCP_READ_ONLY_MODE": "false",`)
		fmt.Println(`  "MCP_CACHE_ENABLED": "true"`)
		fmt.Println(`}`)
		fmt.Println()
		fmt.Println("Secret data (templated variables that would be resolved):")
		fmt.Println(`{`)
		fmt.Println(`  "FIREWALLA_MSP_ID": "your-msp-id-here",`)
		fmt.Println(`  "FIREWALLA_BOX_ID": "your-box-id-here",`)
		fmt.Println(`  "FIREWALLA_MSP_TOKEN": "your-msp-token-here"`)
		fmt.Println(`}`)

	case "apify-mcp-server":
		fmt.Println("ConfigMap data (static environment variables):")
		fmt.Println(`{`)
		fmt.Println(`  "ENABLE_ADDING_ACTORS": "false"`)
		fmt.Println(`}`)
		fmt.Println()
		fmt.Println("Secret data (templated variables that would be resolved):")
		fmt.Println(`{`)
		fmt.Println(`  "ACTORS": "apify/web-scraper,apify/cheerio-scraper",`)
		fmt.Println(`  "TOOLS": "apify-slash-rag-web-browser,search-actors",`)
		fmt.Println(`  "APIFY_TOKEN": "your-apify-token-here"`)
		fmt.Println(`}`)

	default:
		fmt.Printf("No example available for server '%s'\n", serverName)
		fmt.Println("\nGeneric example:")
		fmt.Println("ConfigMap data (static environment variables):")
		fmt.Println(`{`)
		fmt.Println(`  "STATIC_ENV_VAR": "value",`)
		fmt.Println(`  "ENABLE_FEATURE": "true"`)
		fmt.Println(`}`)
		fmt.Println()
		fmt.Println("Secret data (templated variables that would be resolved):")
		fmt.Println(`{`)
		fmt.Println(`  "API_KEY": "your-api-key",`)
		fmt.Println(`  "DATABASE_URL": "your-db-connection-string"`)
		fmt.Println(`}`)
	}

	fmt.Println("\nUsage:")
	fmt.Printf("  Create ConfigMap: %s create-configmap my-config '<configmap-json>' default\n", os.Args[0])
	fmt.Printf("  Create Secret:    %s create-secret my-secrets '<secret-json>' default\n", os.Args[0])

	return nil
}

// extractRealData extracts actual ConfigMap and Secret data from docker mcp catalog
func extractRealData(serverName string) error {
	fmt.Printf("Extracting real data for server '%s' from Docker MCP catalog...\n\n", serverName)

	// Execute docker mcp catalog show --format=json
	cmd := exec.Command("docker", "mcp", "catalog", "show", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute docker mcp catalog command: %w", err)
	}

	// Parse the catalog JSON
	type CatalogServer struct {
		Env []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"env"`
		Secrets []struct {
			Name string `json:"name"`
			Env  string `json:"env"`
		} `json:"secrets"`
	}

	type Catalog struct {
		Registry map[string]CatalogServer `json:"registry"`
	}

	var catalog Catalog
	if err := json.Unmarshal(output, &catalog); err != nil {
		return fmt.Errorf("failed to parse catalog JSON: %w", err)
	}

	// Find the server
	server, exists := catalog.Registry[serverName]
	if !exists {
		return fmt.Errorf("server '%s' not found in catalog", serverName)
	}

	// Separate templated and static environment variables
	staticEnvVars := make(map[string]string)
	templatedEnvVars := make(map[string]string)
	templateVarNames := make(map[string]string) // env var name -> template variable name

	for _, env := range server.Env {
		if strings.Contains(env.Value, "{{") && strings.Contains(env.Value, "}}") {
			// Extract the template variable name from {{variable.name}}
			templateVar := extractTemplateVariable(env.Value)
			templatedEnvVars[env.Name] = "<REPLACE_WITH_ACTUAL_VALUE>"
			templateVarNames[env.Name] = templateVar
		} else {
			// This is static - can be included directly
			staticEnvVars[env.Name] = env.Value
		}
	}

	// Add secrets to templated variables (they also need user-provided values)
	secretEnvVars := make(map[string]string)
	secretVarNames := make(map[string]string) // env var name -> secret variable name
	for _, secret := range server.Secrets {
		secretEnvVars[secret.Env] = "<REPLACE_WITH_ACTUAL_SECRET>"
		secretVarNames[secret.Env] = secret.Name // The secret name IS the template variable name
	}

	// Display ConfigMap data (static env vars)
	fmt.Println("ConfigMap data (static environment variables):")
	if len(staticEnvVars) > 0 {
		configMapJSON, _ := json.MarshalIndent(staticEnvVars, "", "  ")
		fmt.Println(string(configMapJSON))
	} else {
		fmt.Println("{}") // Empty ConfigMap if no static env vars
	}

	fmt.Println()

	// Display Secret data (templated env vars + secrets)
	fmt.Println("Secret data (templated variables + secrets that need actual values):")
	allSecretData := make(map[string]string)
	// Add templated env vars
	for k, v := range templatedEnvVars {
		allSecretData[k] = v
	}
	// Add secret env vars
	for k, v := range secretEnvVars {
		allSecretData[k] = v
	}

	if len(allSecretData) > 0 {
		secretJSON, _ := json.MarshalIndent(allSecretData, "", "  ")
		fmt.Println(string(secretJSON))
	} else {
		fmt.Println("{}") // Empty Secret if no templated/secret vars
	}

	// Show the actual template variable names that need to be configured
	if len(templateVarNames) > 0 || len(secretVarNames) > 0 {
		fmt.Println()
		fmt.Println("Template variable names to configure (these are the keys you need to set):")
		for envVar, templateVar := range templateVarNames {
			fmt.Printf("  %s -> %s\n", envVar, templateVar)
		}
		for envVar, secretVar := range secretVarNames {
			fmt.Printf("  %s -> %s (secret)\n", envVar, secretVar)
		}

		// Show .env file format for --secrets flag
		fmt.Println()
		fmt.Println(".env file format (for use with --secrets flag):")
		fmt.Println("# Create a .env file with these template variable assignments:")
		for _, templateVar := range templateVarNames {
			fmt.Printf("%s=<REPLACE_WITH_ACTUAL_VALUE>\n", templateVar)
		}
		for _, secretVar := range secretVarNames {
			fmt.Printf("%s=<REPLACE_WITH_ACTUAL_SECRET>\n", secretVar)
		}
	}

	fmt.Println()
	fmt.Println("Commands to create resources:")
	fmt.Printf("  ConfigMap: %s create-configmap my-mcp-config '<configmap-json>' default\n", os.Args[0])
	fmt.Printf("  Secret:    %s create-secret my-mcp-secrets '<secret-json>' default\n", os.Args[0])
	fmt.Println()
	fmt.Println("Usage with MCP Gateway:")
	fmt.Println("  Cluster provider mode:")
	fmt.Println("    docker-mcp gateway run \\")
	fmt.Println("      --cluster-config-provider cluster \\")
	fmt.Println("      --cluster-secret-provider cluster \\")
	fmt.Println("      --cluster-config-name my-mcp-config \\")
	fmt.Println("      --cluster-secret-name my-mcp-secrets")
	fmt.Println()
	fmt.Println("  .env file mode:")
	fmt.Println("    docker-mcp gateway run \\")
	fmt.Println("      --secrets /path/to/your/.env")

	return nil
}

// generateSeparateEnvFiles generates separate .env files for configs and secrets
func generateSeparateEnvFiles(serverNames, baseName string) error {
	// Parse comma-delimited server names
	serverList := strings.Split(serverNames, ",")
	for i, server := range serverList {
		serverList[i] = strings.TrimSpace(server)
	}

	fmt.Printf("Generating separate config and secret .env files for servers: %s...\n", strings.Join(serverList, ", "))

	// Execute docker mcp catalog show --format=json
	cmd := exec.Command("docker", "mcp", "catalog", "show", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute docker mcp catalog command: %w", err)
	}

	// Parse the catalog JSON
	type CatalogServer struct {
		Env []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"env"`
		Secrets []struct {
			Name string `json:"name"`
			Env  string `json:"env"`
		} `json:"secrets"`
		Config []struct {
			Name       string                 `json:"name"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"config"`
	}

	type Catalog struct {
		Registry map[string]CatalogServer `json:"registry"`
	}

	var catalog Catalog
	if err := json.Unmarshal(output, &catalog); err != nil {
		return fmt.Errorf("failed to parse catalog JSON: %w", err)
	}

	// Separate config variables and secret variables
	configVars := make(map[string]string)
	secretVars := make(map[string]string)
	configVarSet := make(map[string]bool)
	secretVarSet := make(map[string]bool)

	for _, serverName := range serverList {
		// Find the server
		server, exists := catalog.Registry[serverName]
		if !exists {
			return fmt.Errorf("server '%s' not found in catalog", serverName)
		}

		// Build sets of config and secret parameter names for this server
		configParamNames := make(map[string]bool)
		secretParamNames := make(map[string]bool)

		// Config parameters from the config array
		for _, config := range server.Config {
			if config.Properties != nil {
				for paramName := range config.Properties {
					// Full parameter name includes server prefix
					fullParamName := serverName + "." + paramName
					configParamNames[fullParamName] = true
				}
			}
		}

		// Secret parameters from the secrets array
		for _, secret := range server.Secrets {
			secretParamNames[secret.Name] = true
		}

		// Process environment variables
		for _, env := range server.Env {
			if strings.Contains(env.Value, "{{") && strings.Contains(env.Value, "}}") {
				// Templated variable - check if it's a secret or config
				templateVar := extractTemplateVariable(env.Value)

				if secretParamNames[templateVar] {
					// This template variable corresponds to a secret
					if !secretVarSet[templateVar] {
						secretVars[templateVar] = "<REPLACE_WITH_YOUR_SECRET>"
						secretVarSet[templateVar] = true
					}
				} else {
					// This template variable is a config parameter
					if !configVarSet[templateVar] {
						configVars[templateVar] = "<REPLACE_WITH_YOUR_VALUE>"
						configVarSet[templateVar] = true
					}
				}
			}
			// Static variables are excluded (already in container spec)
		}

		// Process secrets - all go to secret file
		for _, secret := range server.Secrets {
			if !secretVarSet[secret.Name] {
				secretVars[secret.Name] = "<REPLACE_WITH_YOUR_SECRET>"
				secretVarSet[secret.Name] = true
			}
		}
	}

	// Generate file names
	configFile := baseName + "-config.env"
	secretFile := baseName + "-secret.env"

	// Always create config .env file (even if empty for predictable behavior)
	err = createEnvFile(configFile, "config", serverList, configVars)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	fmt.Printf("Generated config file: %s (%d variables)\n", configFile, len(configVars))

	// Always create secret .env file (even if empty for predictable behavior)
	err = createEnvFile(secretFile, "secret", serverList, secretVars)
	if err != nil {
		return fmt.Errorf("failed to create secret file: %w", err)
	}
	fmt.Printf("Generated secret file: %s (%d variables)\n", secretFile, len(secretVars))

	fmt.Printf("\nUsage:\n")
	fmt.Printf("  1. Edit %s (contains static values from catalog)\n", configFile)
	fmt.Printf("  2. Create ConfigMap: ./cluster-tools populate-configmap %s my-mcp-config\n", configFile)
	fmt.Printf("  3. Edit %s and replace placeholders with actual secret values\n", secretFile)
	fmt.Printf("  4. Create Secret: ./cluster-tools populate-secret %s my-mcp-secrets\n", secretFile)
	fmt.Printf("\nThen use with cluster provider mode:\n")
	fmt.Printf("  docker-mcp gateway run \\\n")
	fmt.Printf("    --cluster-config-provider cluster \\\n")
	fmt.Printf("    --cluster-secret-provider cluster \\\n")
	fmt.Printf("    --cluster-config-name my-mcp-config \\\n")
	fmt.Printf("    --cluster-secret-name my-mcp-secrets\n")

	return nil
}

// createEnvFile creates a .env file with the specified variables
func createEnvFile(filename, fileType string, serverList []string, vars map[string]string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer file.Close()

	// Write header comment
	fmt.Fprintf(file, "# MCP Gateway %s variables for servers: %s\n", fileType, strings.Join(serverList, ", "))
	fmt.Fprintf(file, "# Generated by mcp-tool on %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "# \n")

	if fileType == "config" {
		fmt.Fprintf(file, "# Static configuration values from catalog\n")
		fmt.Fprintf(file, "# These values can be modified as needed\n")
		if len(vars) == 0 {
			fmt.Fprintf(file, "# No static config variables found for these servers\n")
		}
	} else {
		fmt.Fprintf(file, "# Secret values - replace placeholders with actual secrets\n")
		fmt.Fprintf(file, "# These are template variables and credentials\n")
		if len(vars) == 0 {
			fmt.Fprintf(file, "# No secret variables found for these servers\n")
		}
	}

	fmt.Fprintf(file, "\n")

	// Write variables
	if len(vars) > 0 {
		for key, value := range vars {
			fmt.Fprintf(file, "%s=%s\n", key, value)
		}
	} else {
		fmt.Fprintf(file, "# (Empty - no variables to configure)\n")
	}

	return nil
}

// populateConfigFromEnvFile reads a config .env file and creates a ConfigMap
func populateConfigFromEnvFile(ctx context.Context, clientset kubernetes.Interface, envFile, configName, namespace string) error {
	fmt.Printf("Creating ConfigMap from %s...\n", envFile)

	envVars, err := readEnvFile(envFile)
	if err != nil {
		return fmt.Errorf("failed to read env file: %w", err)
	}

	// Create ConfigMap (even if empty for predictable behavior)
	configJSON, _ := json.Marshal(envVars)
	err = createConfigMap(ctx, clientset, configName, string(configJSON), namespace)
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	fmt.Printf("Successfully created ConfigMap '%s' with %d variables in namespace '%s'\n", configName, len(envVars), namespace)
	return nil
}

// populateSecretFromEnvFile reads a secret .env file and creates a Secret
func populateSecretFromEnvFile(ctx context.Context, clientset kubernetes.Interface, envFile, secretName, namespace string) error {
	fmt.Printf("Creating Secret from %s...\n", envFile)

	envVars, err := readEnvFile(envFile)
	if err != nil {
		return fmt.Errorf("failed to read env file: %w", err)
	}

	// Filter out placeholder values
	secretVars := make(map[string]string)
	for key, value := range envVars {
		if value == "" || value == "<REPLACE_WITH_YOUR_VALUE>" || value == "<REPLACE_WITH_YOUR_SECRET>" {
			fmt.Printf("Warning: Skipping %s (empty or placeholder value)\n", key)
			continue
		}
		secretVars[key] = value
	}

	// Create Secret (even if empty for predictable behavior)
	secretJSON, _ := json.Marshal(secretVars)
	err = createSecret(ctx, clientset, secretName, string(secretJSON), namespace)
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}

	if len(secretVars) == 0 {
		fmt.Printf("Successfully created empty Secret '%s' in namespace '%s' (all values were placeholders)\n", secretName, namespace)
	} else {
		fmt.Printf("Successfully created Secret '%s' with %d variables in namespace '%s'\n", secretName, len(secretVars), namespace)
	}
	return nil
}

// extractTemplateVariable extracts the variable name from a template expression like {{variable.name}}
func extractTemplateVariable(templateExpr string) string {
	// Remove {{ and }} and any whitespace
	templateExpr = strings.TrimSpace(templateExpr)
	if strings.HasPrefix(templateExpr, "{{") && strings.HasSuffix(templateExpr, "}}") {
		// Extract content between {{ }}
		content := strings.TrimSpace(templateExpr[2 : len(templateExpr)-2])
		// Handle pipeline expressions like {{path|filter}} - take only the first part
		if pipeIndex := strings.Index(content, "|"); pipeIndex != -1 {
			content = strings.TrimSpace(content[:pipeIndex])
		}
		return content
	}
	return templateExpr // Return as-is if not a proper template
}

// getKeys returns the keys of a map as a slice
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// readEnvFile reads and parses a .env file, similar to the MCP gateway's implementation
func readEnvFile(envFile string) (map[string]string, error) {
	envVars := make(map[string]string)

	buf, err := os.ReadFile(envFile)
	if err != nil {
		return nil, fmt.Errorf("reading env file %s: %w", envFile, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid line in env file: %s", line)
		}

		envVars[key] = value
	}

	return envVars, nil
}
