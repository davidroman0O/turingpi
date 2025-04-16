package cmd

import (
	"fmt"
	"os"

	"github.com/davidroman0O/turingpi/pkg/imageprep" // Adjust import path if needed
	"github.com/spf13/cobra"
)

var (
	prepSourceImg    string
	prepNodeNum      int
	prepNodeIPCIDR   string
	prepNodeHostname string
	prepNodeGateway  string
	prepNodeDNS      string
	prepCacheDir     string
)

// prepareCmd represents the prepare command
var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare an OS image with node-specific configuration",
	Long: `Decompresses an OS image (.img.xz), mounts its root filesystem,
modifies configuration files (hostname, static IP via Netplan),
unmounts, and recompresses the image to a cache directory.

Requires 'sudo' privileges for mount/kpartx operations.
Requires 'kpartx' and 'xz-utils' to be installed.

Example:
sudo ./turingpi prepare \
    --source ./images/ubuntu-original.img.xz \
    --node 1 \
    --ip "192.168.1.101/24" \
    --hostname "rk1-node1" \
    --gateway "192.168.1.1" \
    --dns "1.1.1.1,8.8.8.8" \
    --cache-dir ./prepared-images
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Executing prepare command...")

		// Basic check for sudo - this should ideally be run via sudo
		if os.Geteuid() != 0 {
			fmt.Fprintln(os.Stderr, "Error: This command requires sudo privileges to run.")
			fmt.Fprintln(os.Stderr, "Please run using 'sudo ./turingpi prepare ...'")
			os.Exit(1)
		}

		// Default hostname if not provided
		if prepNodeHostname == "" {
			prepNodeHostname = fmt.Sprintf("rk1-node%d", prepNodeNum)
			fmt.Printf("Hostname not provided, defaulting to: %s\n", prepNodeHostname)
		}

		opts := imageprep.PrepareImageOptions{
			SourceImgXZ:    prepSourceImg,
			NodeNum:        prepNodeNum,
			NodeIPCIDR:     prepNodeIPCIDR,
			NodeHostname:   prepNodeHostname,
			NodeGateway:    prepNodeGateway,
			NodeDNSServers: prepNodeDNS,
			CacheDir:       prepCacheDir,
		}

		preparedPath, err := imageprep.PrepareImage(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error preparing image: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully prepared image: %s\n", preparedPath)
	},
}

func init() {
	rootCmd.AddCommand(prepareCmd)

	prepareCmd.Flags().StringVarP(&prepSourceImg, "source", "s", "", "Path to the source OS image (.img.xz)")
	prepareCmd.Flags().IntVarP(&prepNodeNum, "node", "n", 0, "Target node number (used for default hostname)")
	prepareCmd.Flags().StringVar(&prepNodeIPCIDR, "ip", "", "Static IP address and CIDR (e.g., 192.168.1.101/24)")
	prepareCmd.Flags().StringVar(&prepNodeHostname, "hostname", "", "Hostname to set for the node (defaults to rk1-node<N>)")
	prepareCmd.Flags().StringVar(&prepNodeGateway, "gateway", "", "Gateway IP address")
	prepareCmd.Flags().StringVar(&prepNodeDNS, "dns", "", "Comma-separated list of DNS server IPs")
	prepareCmd.Flags().StringVar(&prepCacheDir, "cache-dir", "./prepared-images", "Directory to store/check for prepared images")

	// Mark flags as required
	_ = prepareCmd.MarkFlagRequired("source")
	_ = prepareCmd.MarkFlagRequired("node") // Still needed for default hostname
	_ = prepareCmd.MarkFlagRequired("ip")
	_ = prepareCmd.MarkFlagRequired("gateway")
	_ = prepareCmd.MarkFlagRequired("dns")
	// Hostname is optional (defaults)
	// CacheDir has a default
}
