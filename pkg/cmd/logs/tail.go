package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/config"
	"github.com/stripe/stripe-cli/pkg/logs"
	"github.com/stripe/stripe-cli/pkg/validators"
)

const requestLogsWebSocketFeature = "request_logs"

// LogsTailCmd wraps the configuration for the tail command
type LogsTailCmd struct {
	apiBaseURL string
	cfg        *config.Config
	Cmd        *cobra.Command
	format     string
	LogFilters *logs.LogFilters
	noWSS      bool
}

// NewLogsTailCmd creates and initializes the tail command for the logs package
func NewLogsTailCmd(config *config.Config) *LogsTailCmd {
	tailCmd := &LogsTailCmd{
		cfg: config,
		LogFilters: &logs.LogFilters{},
	}

	tailCmd.Cmd = &cobra.Command{
		Use:   "tail",
		Args:  validators.NoArgs,
		Short: "Listens for API request logs sent from Stripe to help test your integration.",
		Long: fmt.Sprintf(`
The tail command lets you tail API request logs from Stripe.
The command establishes a direct connection with Stripe to send the request logs to your local machine.

Watch for all request logs sent from Stripe:

  $ stripe logs tail`),
		RunE: tailCmd.runTailCmd,
	}

	tailCmd.Cmd.Flags().StringVar(&tailCmd.format, "format", "default", "Specifies the output format of request logs")

	// Log filters
	tailCmd.Cmd.Flags().StringVar(&tailCmd.LogFilters.FilterIPAddress, "filter-ip-address", "", "Filter request logs by ip address")
	tailCmd.Cmd.Flags().StringVar(&tailCmd.LogFilters.FilterHTTPMethod, "filter-http-method", "", "Filter request logs by http method")
	tailCmd.Cmd.Flags().StringVar(&tailCmd.LogFilters.FilterRequestPath, "filter-request-path", "", "Filter request logs by request path")
	tailCmd.Cmd.Flags().StringVar(&tailCmd.LogFilters.FilterSource, "filter-source", "", "Filter request logs by source (dashboard or API)")
	tailCmd.Cmd.Flags().StringVar(&tailCmd.LogFilters.FilterStatusCode, "filter-status-code", "", "Filter request logs by status code")
	tailCmd.Cmd.Flags().StringVar(&tailCmd.LogFilters.FilterStatusCodeType, "filter-status-code-type", "", "Filter request logs by status code type")

	// Hidden configuration flags, useful for dev/debugging
	tailCmd.Cmd.Flags().StringVar(&tailCmd.apiBaseURL, "api-base", "", "Sets the API base URL")
	tailCmd.Cmd.Flags().MarkHidden("api-base") // #nosec G104

	tailCmd.Cmd.Flags().BoolVar(&tailCmd.noWSS, "no-wss", false, "Force unencrypted ws:// protocol instead of wss://")
	tailCmd.Cmd.Flags().MarkHidden("no-wss") // #nosec G104

	return tailCmd
}

func (tailCmd *LogsTailCmd) runTailCmd(cmd *cobra.Command, args []string) error {
	deviceName, err := tailCmd.cfg.Profile.GetDeviceName()
	if err != nil {
		return err
	}

	key, err := tailCmd.cfg.Profile.GetSecretKey()
	if err != nil {
		return err
	}

	tailer := logs.New(&logs.Config{
		APIBaseURL:       tailCmd.apiBaseURL,
		DeviceName:       deviceName,
		Filters:          tailCmd.LogFilters,
		Key:              key,
		Log:              log.StandardLogger(),
		NoWSS:            tailCmd.noWSS,
		OutputFormat:     tailCmd.format,
		WebSocketFeature: requestLogsWebSocketFeature,
	})

	err = tailer.Run()
	if err != nil {
		return err
	}

	return nil
}