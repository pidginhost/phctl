package freedns

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

var (
	outputFormat = cmdutil.OutputFormat
	force        = cmdutil.Force
)

var Cmd = &cobra.Command{
	Use:     "freedns",
	Aliases: []string{"fdns"},
	Short:   "Manage FreeDNS domains and records",
	Args:    cobra.NoArgs,
}

// --- Domains ---

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage FreeDNS domains",
	Args:  cobra.NoArgs,
}

var domainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List FreeDNS domains",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		domains, _, err := c.FreednsAPI.FreednsDnsList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing FreeDNS domains: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, domains, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "DOMAIN", "ACTIVE", "SOURCE")
			for _, d := range domains {
				output.PrintRow(tw, d.Domain, d.Active, d.Source)
			}
			tw.Flush()
		})
	},
}

var (
	activateSource string
	activateIP     string
)

var domainActivateCmd = &cobra.Command{
	Use:   "activate <domain>",
	Short: "Activate FreeDNS for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewActivateFreeDNS(args[0], pidginhost.SourceEnum(activateSource), activateIP)
		resp, _, err := c.FreednsAPI.FreednsDnsActivateCreate(context.Background()).ActivateFreeDNS(body).Execute()
		if err != nil {
			return fmt.Errorf("activating FreeDNS: %w", err)
		}
		fmt.Printf("FreeDNS activated: %s\n", resp.Message)
		return nil
	},
}

var deactivateSource string

var domainDeactivateCmd = &cobra.Command{
	Use:   "deactivate <domain>",
	Short: "Deactivate FreeDNS for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Deactivate FreeDNS for %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDeactivateFreeDNS(args[0], pidginhost.SourceEnum(deactivateSource))
		resp, _, err := c.FreednsAPI.FreednsDnsDeactivateCreate(context.Background()).DeactivateFreeDNS(body).Execute()
		if err != nil {
			return fmt.Errorf("deactivating FreeDNS: %w", err)
		}
		fmt.Printf("FreeDNS deactivated: %s\n", resp.Message)
		return nil
	},
}

// --- Records ---

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Manage DNS records",
	Args:  cobra.NoArgs,
}

var (
	recordListDomain string
	recordListSource string
)

var recordListCmd = &cobra.Command{
	Use:   "list",
	Short: "List DNS records for a domain",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		req := c.FreednsAPI.FreednsDnsRecordsList(context.Background()).Domain(recordListDomain)
		if recordListSource != "" {
			req = req.Source(recordListSource)
		}
		records, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("listing records: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, records, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "LINE", "TYPE", "NAME", "TTL", "ADDRESS")
			for _, r := range records {
				output.PrintRow(tw, r.Line, r.Type, r.Name, r.Ttl, r.Address)
			}
			tw.Flush()
		})
	},
}

var (
	recordCreateDomain string
	recordCreateSource string
	recordType         string
	recordName         string
	recordTTL          int32
	recordAddress      string
)

var recordCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a DNS record",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDNSRecordCreate(recordName, recordTTL, pidginhost.DNSRecordCreateTypeEnum(recordType))
		if recordAddress != "" {
			body.Address = &recordAddress
		}
		req := c.FreednsAPI.FreednsDnsAddRecordCreate(context.Background()).Domain(recordCreateDomain).DNSRecordCreate(body)
		if recordCreateSource != "" {
			req = req.Source(recordCreateSource)
		}
		resp, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("creating record: %w", err)
		}
		fmt.Printf("Record created: %s\n", resp.Message)
		return nil
	},
}

var (
	recordDeleteDomain string
	recordDeleteSource string
	recordDeleteLine   int32
)

var recordDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a DNS record",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete record line %d from %s?", recordDeleteLine, recordDeleteDomain)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDeleteRecord(recordDeleteLine)
		req := c.FreednsAPI.FreednsDnsDeleteRecordCreate(context.Background()).Domain(recordDeleteDomain).DeleteRecord(body)
		if recordDeleteSource != "" {
			req = req.Source(recordDeleteSource)
		}
		resp, _, err := req.Execute()
		if err != nil {
			return fmt.Errorf("deleting record: %w", err)
		}
		fmt.Printf("Record deleted: %s\n", resp.Message)
		return nil
	},
}

func init() {
	domainActivateCmd.Flags().StringVar(&activateSource, "source", "internal", "Source type: internal or external (required)")
	domainActivateCmd.Flags().StringVar(&activateIP, "ip", "", "IP address (required)")
	domainActivateCmd.MarkFlagRequired("ip")

	domainDeactivateCmd.Flags().StringVar(&deactivateSource, "source", "internal", "Source type: internal or external")

	recordListCmd.Flags().StringVar(&recordListDomain, "domain", "", "Domain name (required)")
	recordListCmd.Flags().StringVar(&recordListSource, "source", "", "Source: internal or external")
	recordListCmd.MarkFlagRequired("domain")

	recordCreateCmd.Flags().StringVar(&recordCreateDomain, "domain", "", "Domain name (required)")
	recordCreateCmd.Flags().StringVar(&recordCreateSource, "source", "", "Source: internal or external")
	recordCreateCmd.Flags().StringVar(&recordType, "type", "", "Record type: A, AAAA, CNAME, MX, TXT, SRV, CAA (required)")
	recordCreateCmd.Flags().StringVar(&recordName, "name", "", "Record name (required)")
	recordCreateCmd.Flags().Int32Var(&recordTTL, "ttl", 3600, "TTL in seconds")
	recordCreateCmd.Flags().StringVar(&recordAddress, "address", "", "Record address/value")
	recordCreateCmd.MarkFlagRequired("domain")
	recordCreateCmd.MarkFlagRequired("type")
	recordCreateCmd.MarkFlagRequired("name")

	recordDeleteCmd.Flags().StringVar(&recordDeleteDomain, "domain", "", "Domain name (required)")
	recordDeleteCmd.Flags().StringVar(&recordDeleteSource, "source", "", "Source: internal or external")
	recordDeleteCmd.Flags().Int32Var(&recordDeleteLine, "line", 0, "Record line number (required)")
	recordDeleteCmd.MarkFlagRequired("domain")
	recordDeleteCmd.MarkFlagRequired("line")

	domainCmd.AddCommand(domainListCmd)
	domainCmd.AddCommand(domainActivateCmd)
	domainCmd.AddCommand(domainDeactivateCmd)

	recordCmd.AddCommand(recordListCmd)
	recordCmd.AddCommand(recordCreateCmd)
	recordCmd.AddCommand(recordDeleteCmd)

	Cmd.AddCommand(domainCmd)
	Cmd.AddCommand(recordCmd)
}
