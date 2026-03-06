package domain

import (
	"context"
	"fmt"
	"io"
	"strings"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/confirm"
	"github.com/pidginhost/phctl/internal/output"
)

// Local types to bypass SDK float64 vs string mismatches for decimal fields.

type rawTLD struct {
	Id        int32  `json:"id"`
	Tld       string `json:"tld"`
	Price     string `json:"price"`
	Registrar string `json:"registrar"`
}

type rawDomain struct {
	Id             int32   `json:"id"`
	Domain         string  `json:"domain"`
	Idna           string  `json:"idna"`
	Tld            rawTLD  `json:"tld"`
	Nameservers    *string `json:"nameservers"`
	ExpirationDate string  `json:"expiration_date"`
	ServiceStatus  string  `json:"service_status"`
	MaxRenewYears  int32   `json:"max_renew_years"`
}

var (
	outputFormat = cmdutil.OutputFormat
	force        = cmdutil.Force
)

var Cmd = &cobra.Command{
	Use:     "domain",
	Aliases: []string{"dns"},
	Short:   "Manage domains, registrants, and TLDs",
	Args:    cobra.NoArgs,
}

// --- Domain CRUD ---

var domainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all domains",
	RunE: func(cmd *cobra.Command, args []string) error {
		domains, err := client.RawFetchAll[rawDomain]("/api/domain/domain/")
		if err != nil {
			return fmt.Errorf("listing domains: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, domains, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "DOMAIN", "TLD", "EXPIRATION", "STATUS")
			for _, d := range domains {
				output.PrintRow(tw, d.Id, d.Domain, d.Tld.Tld, d.ExpirationDate, d.ServiceStatus)
			}
			tw.Flush()
		})
	},
}

var domainGetCmd = &cobra.Command{
	Use:   "get <domain>",
	Short: "Get domain details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var d rawDomain
		if err := client.RawGet(fmt.Sprintf("/api/domain/domain/%s/", args[0]), &d); err != nil {
			return fmt.Errorf("getting domain: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, d, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", d.Id)
			output.PrintRow(tw, "Domain:", d.Domain)
			output.PrintRow(tw, "TLD:", d.Tld.Tld)
			output.PrintRow(tw, "IDNA:", d.Idna)
			output.PrintRow(tw, "Nameservers:", pstr(d.Nameservers))
			output.PrintRow(tw, "Expiration:", d.ExpirationDate)
			output.PrintRow(tw, "Status:", d.ServiceStatus)
			output.PrintRow(tw, "Max Renew Years:", d.MaxRenewYears)
			tw.Flush()
		})
	},
}

var (
	domainCreateNameservers string
	domainCreateYears       int32
)

var domainCreateCmd = &cobra.Command{
	Use:   "create <domain>",
	Short: "Register a new domain",
	Long:  "Register a new domain. Example: phctl domain create example.ro",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Register domain %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDomainCreate(args[0])
		if domainCreateNameservers != "" {
			body.Nameservers = pidginhost.PtrString(domainCreateNameservers)
		}
		if domainCreateYears > 0 {
			body.Years = pidginhost.PtrInt32(domainCreateYears)
		}
		resp, _, err := c.DomainAPI.DomainDomainCreate(context.Background()).DomainCreate(body).Execute()
		if err != nil {
			return fmt.Errorf("registering domain: %w", err)
		}
		fmt.Printf("Domain registered: %s\n", resp.Domain)
		return nil
	},
}

var domainCheckCmd = &cobra.Command{
	Use:   "check <domain>",
	Short: "Check domain availability",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewCheckAvailability(args[0])
		resp, _, err := c.DomainAPI.DomainDomainCheckAvailabilityCreate(context.Background()).CheckAvailability(body).Execute()
		if err != nil {
			return fmt.Errorf("checking availability: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, resp, func(w io.Writer) {
			fmt.Fprintf(w, "Domain: %s\n", resp.Domain)
		})
	},
}

var domainRenewYears int32

var domainRenewCmd = &cobra.Command{
	Use:   "renew <domain>",
	Short: "Renew a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Renew domain %s for %d year(s)?", args[0], domainRenewYears)) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewRenewDomain(domainRenewYears)
		_, _, err = c.DomainAPI.DomainDomainRenewCreate(context.Background(), args[0]).RenewDomain(body).Execute()
		if err != nil {
			return fmt.Errorf("renewing domain: %w", err)
		}
		fmt.Printf("Domain %s renewed for %d year(s).\n", args[0], domainRenewYears)
		return nil
	},
}

var domainCancelCmd = &cobra.Command{
	Use:   "cancel <domain>",
	Short: "Cancel a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Cancel domain %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.DomainAPI.DomainDomainCancelCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("cancelling domain: %w", err)
		}
		fmt.Printf("Domain %s cancelled.\n", args[0])
		return nil
	},
}

var transferAuthCode string

var domainTransferCmd = &cobra.Command{
	Use:   "transfer <domain>",
	Short: "Transfer a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Transfer domain %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewTransferRoDomain(args[0], transferAuthCode)
		resp, _, err := c.DomainAPI.DomainDomainTransferRoDomainCreate(context.Background()).TransferRoDomain(body).Execute()
		if err != nil {
			return fmt.Errorf("transferring domain: %w", err)
		}
		fmt.Printf("Domain transfer initiated: %s\n", resp.Domain)
		return nil
	},
}

var nameserversValue string

var domainNameserversCmd = &cobra.Command{
	Use:   "nameservers <domain>",
	Short: "Update nameservers for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewNameserversUpdate(strings.Split(nameserversValue, ","))
		_, _, err = c.DomainAPI.DomainDomainNameserversCreate(context.Background(), args[0]).NameserversUpdate(body).Execute()
		if err != nil {
			return fmt.Errorf("updating nameservers: %w", err)
		}
		fmt.Printf("Nameservers updated for %s.\n", args[0])
		return nil
	},
}

// --- TLD ---

var tldCmd = &cobra.Command{
	Use:   "tld",
	Short: "List available TLDs",
	Args:  cobra.NoArgs,
}

var tldListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available TLDs",
	RunE: func(cmd *cobra.Command, args []string) error {
		tlds, err := client.RawFetchAll[rawTLD]("/api/domain/tld/")
		if err != nil {
			return fmt.Errorf("listing TLDs: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, tlds, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "TLD", "PRICE", "REGISTRAR")
			for _, t := range tlds {
				output.PrintRow(tw, t.Id, t.Tld, t.Price, t.Registrar)
			}
			tw.Flush()
		})
	},
}

// --- Registrants ---

var registrantCmd = &cobra.Command{
	Use:   "registrant",
	Short: "Manage domain registrants",
	Args:  cobra.NoArgs,
}

var registrantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registrants",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		registrants, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.DomainRegistrant, bool, error) {
			resp, _, err := c.DomainAPI.DomainRegistrantsList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing registrants: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, registrants, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "FIRST NAME", "LAST NAME", "EMAIL", "COUNTRY", "CITY")
			for _, r := range registrants {
				output.PrintRow(tw, r.Id, r.FirstName, r.LastName, r.Email, r.Country, r.City)
			}
			tw.Flush()
		})
	},
}

var registrantGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get registrant details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		r, _, err := c.DomainAPI.DomainRegistrantsRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting registrant: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, r, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", r.Id)
			output.PrintRow(tw, "Name:", r.FirstName+" "+r.LastName)
			output.PrintRow(tw, "Email:", r.Email)
			output.PrintRow(tw, "Phone:", r.Phone)
			output.PrintRow(tw, "Address:", r.Address)
			output.PrintRow(tw, "City:", r.City)
			output.PrintRow(tw, "Region:", r.Region)
			output.PrintRow(tw, "Postal Code:", r.PostalCode)
			output.PrintRow(tw, "Country:", r.Country)
			tw.Flush()
		})
	},
}

var (
	regFirstName  string
	regLastName   string
	regEmail      string
	regPhone      string
	regAddress    string
	regCity       string
	regRegion     string
	regPostalCode string
	regCountry    string
)

var registrantCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a registrant",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDomainRegistrant(
			0,
			regFirstName,
			regLastName,
			regAddress,
			regCity,
			regRegion,
			regPostalCode,
			pidginhost.CountryEnum(regCountry),
			regEmail,
			regPhone,
		)
		resp, _, err := c.DomainAPI.DomainRegistrantsCreate(context.Background()).DomainRegistrant(body).Execute()
		if err != nil {
			return fmt.Errorf("creating registrant: %w", err)
		}
		fmt.Printf("Registrant created (ID: %d, %s %s)\n", resp.Id, resp.FirstName, resp.LastName)
		return nil
	},
}

var registrantDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a registrant",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Delete registrant %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		_, err = c.DomainAPI.DomainRegistrantsDestroy(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("deleting registrant: %w", err)
		}
		fmt.Printf("Registrant %s deleted.\n", args[0])
		return nil
	},
}

func pstr[T any](p *T) string {
	if p == nil {
		return "<none>"
	}
	return fmt.Sprintf("%v", *p)
}

func init() {
	domainCreateCmd.Flags().StringVar(&domainCreateNameservers, "nameservers", "", "Comma-separated nameservers (2-5)")
	domainCreateCmd.Flags().Int32Var(&domainCreateYears, "years", 0, "Registration period in years")

	domainRenewCmd.Flags().Int32Var(&domainRenewYears, "years", 1, "Renewal period in years")

	domainTransferCmd.Flags().StringVar(&transferAuthCode, "auth-code", "", "Domain transfer auth code (required)")
	domainTransferCmd.MarkFlagRequired("auth-code")

	domainNameserversCmd.Flags().StringVar(&nameserversValue, "nameservers", "", "Comma-separated nameservers (required)")
	domainNameserversCmd.MarkFlagRequired("nameservers")

	registrantCreateCmd.Flags().StringVar(&regFirstName, "first-name", "", "First name (required)")
	registrantCreateCmd.Flags().StringVar(&regLastName, "last-name", "", "Last name (required)")
	registrantCreateCmd.Flags().StringVar(&regEmail, "email", "", "Email (required)")
	registrantCreateCmd.Flags().StringVar(&regPhone, "phone", "", "Phone (required)")
	registrantCreateCmd.Flags().StringVar(&regAddress, "address", "", "Address (required)")
	registrantCreateCmd.Flags().StringVar(&regCity, "city", "", "City (required)")
	registrantCreateCmd.Flags().StringVar(&regRegion, "region", "", "Region (required)")
	registrantCreateCmd.Flags().StringVar(&regPostalCode, "postal-code", "", "Postal code (required)")
	registrantCreateCmd.Flags().StringVar(&regCountry, "country", "", "Country code, e.g. RO (required)")
	registrantCreateCmd.MarkFlagRequired("first-name")
	registrantCreateCmd.MarkFlagRequired("last-name")
	registrantCreateCmd.MarkFlagRequired("email")
	registrantCreateCmd.MarkFlagRequired("phone")
	registrantCreateCmd.MarkFlagRequired("address")
	registrantCreateCmd.MarkFlagRequired("city")
	registrantCreateCmd.MarkFlagRequired("region")
	registrantCreateCmd.MarkFlagRequired("postal-code")
	registrantCreateCmd.MarkFlagRequired("country")

	tldCmd.AddCommand(tldListCmd)

	registrantCmd.AddCommand(registrantListCmd)
	registrantCmd.AddCommand(registrantGetCmd)
	registrantCmd.AddCommand(registrantCreateCmd)
	registrantCmd.AddCommand(registrantDeleteCmd)

	Cmd.AddCommand(domainListCmd)
	Cmd.AddCommand(domainGetCmd)
	Cmd.AddCommand(domainCreateCmd)
	Cmd.AddCommand(domainCheckCmd)
	Cmd.AddCommand(domainRenewCmd)
	Cmd.AddCommand(domainCancelCmd)
	Cmd.AddCommand(domainTransferCmd)
	Cmd.AddCommand(domainNameserversCmd)
	Cmd.AddCommand(tldCmd)
	Cmd.AddCommand(registrantCmd)
}
