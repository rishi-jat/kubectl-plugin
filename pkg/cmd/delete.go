package cmd

import (
	"fmt"
	"strings"
	"bufio"
	"os"

	"kubectl-multi/pkg/cluster"
	"kubectl-multi/pkg/util"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// Custom help function for delete command
func deleteHelpFunc(cmd *cobra.Command, args []string) {
	// Get original kubectl help using the new implementation
	cmdInfo, err := util.GetKubectlCommandInfo("delete")
	if err != nil {
		// Fallback to default help if kubectl help is not available
		cmd.Help()
		return
	}

	// Multi-cluster plugin information
	multiClusterInfo := `Delete resources across all managed clusters.
This command deletes resources from all KubeStellar managed clusters.`

	// Multi-cluster examples
	multiClusterExamples := `# Delete a deployment from all managed clusters
kubectl multi delete deployment nginx

# Delete pods with a specific label from all clusters
kubectl multi delete pods -l app=nginx

# Delete resources from a file across all clusters
kubectl multi delete -f deployment.yaml

# Delete all pods in all clusters
kubectl multi delete pods --all

# Delete with force flag across all clusters
kubectl multi delete pod nginx --force`

	// Multi-cluster usage
	multiClusterUsage := `kubectl multi delete [TYPE[.VERSION][.GROUP] [NAME | -l label] | TYPE[.VERSION][.GROUP]/NAME ...] [flags]`

	// Format combined help using the new CommandInfo structure
	combinedHelp := util.FormatMultiClusterHelp(cmdInfo, multiClusterInfo, multiClusterExamples, multiClusterUsage)
	fmt.Fprintln(cmd.OutOrStdout(), combinedHelp)
}

func newDeleteCommand() *cobra.Command {
	var filename string
	var recursive bool
	var dryRun string

	cmd := &cobra.Command{
		Use:   "delete [TYPE[.VERSION][.GROUP] [NAME | -l label] | TYPE[.VERSION][.GROUP]/NAME ...]",
		Short: "Delete resources across all managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {

			kubeconfig, remoteCtx, _, namespace, allNamespaces := GetGlobalFlags()
			return handleDeleteCommand(args, filename, recursive, dryRun, kubeconfig, remoteCtx, namespace, allNamespaces)
		},
	}

	cmd.Flags().StringVarP(&filename, "filename", "f", "", "filename, directory, or URL to files to use to delete the resource")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "process the directory used in -f, --filename recursively")
	cmd.Flags().StringVar(&dryRun, "dry-run", "none", "must be \"none\", \"server\", or \"client\"")

	// Set custom help function
	cmd.SetHelpFunc(deleteHelpFunc)

	return cmd
}

func handleDeleteCommand(args []string, filename string, recursive bool, dryRun, kubeconfig, remoteCtx, namespace string, allNamespaces bool) error {

	var isFileProvided bool
	var resourceName string
	var resourceType string

	if len(args) != 0 && filename != "" {
		return fmt.Errorf("provide either filename or resource type at a time")
	}

	if filename != "" {
		isFileProvided = true
	} else {
		isFileProvided = false // in this case reource type is provided.
		resourceType = args[0]
		resourceName = ""
		if len(args) > 1 {
			resourceName = args[1]
		}
	}

	clusters, err := cluster.DiscoverClusters(kubeconfig, remoteCtx)
	if err != nil {
		return fmt.Errorf("failed to discover clusters: %v", err)
	}
	if len(clusters) == 0 {
		return fmt.Errorf("no clusters discovered")
	}

	fmt.Println("Are you sure you want to delete these resources ?")
	fmt.Println("Type 'yes' to confirm, or anything else to cancel.")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')

	if err != nil {
		return fmt.Errorf("failed to read confirmation: %v", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "yes" {
		fmt.Println("Deletion cancelled...")
		return nil
	}

	// Find current context from kubeconfig
	currentContext := ""
	{
		loading := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			loading.ExplicitPath = kubeconfig
		}
		cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{})
		rawCfg, err := cfg.RawConfig()
		if err == nil {
			currentContext = rawCfg.CurrentContext
		}
	}

	// Identify ITS (control) cluster context
	itsContext := remoteCtx

	// Build maps for quick lookup
	contextToCluster := make(map[string]cluster.ClusterInfo)
	for _, c := range clusters {
		contextToCluster[c.Context] = c
	}

	// 1. Run for current context (if present)
	if cinfo, ok := contextToCluster[currentContext]; ok {
		var args []string
		if isFileProvided {
			args = []string{"delete", "-f", filename, "--context", cinfo.Context}
		} else {
			args = []string{"delete", resourceType, resourceName, "--context", cinfo.Context}
		}
		if recursive {
			args = append(args, "-R")
		}
		if dryRun != "none" && dryRun != "" {
			args = append(args, "--dry-run="+dryRun)
		}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		output, err := runKubectl(args, kubeconfig)
		fmt.Printf("=== Cluster: %s ===\n", cinfo.Context)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Print(output)
		}
		fmt.Println()
	}

	// 2. Run for KubeStellar clusters (excluding ITS and current)
	for _, c := range clusters {
		if c.Context == currentContext || c.Context == itsContext {
			continue
		}
		var args []string
		if isFileProvided {
			args = []string{"delete", "-f", filename, "--context", c.Context}
		} else {
			args = []string{"delete", resourceType, resourceName, "--context", c.Context}
		}
		if recursive {
			args = append(args, "-R")
		}
		if dryRun != "none" && dryRun != "" {
			args = append(args, "--dry-run="+dryRun)
		}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		output, err := runKubectl(args, kubeconfig)
		fmt.Printf("=== Cluster: %s ===\n", c.Context)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Print(output)
		}
		fmt.Println()
	}

	// 3. Print warning for ITS (control) cluster
	if cinfo, ok := contextToCluster[itsContext]; ok {
		fmt.Printf("=== Cluster: %s ===\n", cinfo.Context)
		fmt.Printf("Cannot perform this operation on ITS (control) cluster: %s\n", cinfo.Context)
		fmt.Println()
	}

	return nil
}

func newExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec POD [-c CONTAINER] -- COMMAND [args...]",
		Short: "Execute a command in a container across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("exec command not yet implemented")
		},
	}
	return cmd
}

func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create -f FILENAME",
		Short: "Create a resource from a file or from stdin across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("create command not yet implemented")
		},
	}
	return cmd
}

func newEditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [TYPE[.VERSION][.GROUP]/]NAME",
		Short: "Edit a resource on the server across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("edit command not yet implemented")
		},
	}
	return cmd
}

func newPatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch [TYPE[.VERSION][.GROUP]/]NAME --patch PATCH",
		Short: "Update field(s) of a resource across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("patch command not yet implemented")
		},
	}
	return cmd
}

func newScaleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale [TYPE[.VERSION][.GROUP]/]NAME --replicas=COUNT",
		Short: "Set a new size for a deployment, replica set, or stateful set across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("scale command not yet implemented")
		},
	}
	return cmd
}

func newPortForwardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward POD [LOCAL_PORT:]REMOTE_PORT",
		Short: "Forward one or more local ports to a pod across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("port-forward command not yet implemented")
		},
	}
	return cmd
}

func newTopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top [TYPE]",
		Short: "Display resource (CPU/memory/storage) usage across managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("top command not yet implemented")
		},
	}
	return cmd
}
