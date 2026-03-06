package billing

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

// Local types to bypass SDK float64 vs string mismatches for decimal fields.

type rawFundsBalance struct {
	Balance       string `json:"balance"`
	ThresholdType string `json:"threshold_type"`
}

type rawDeposit struct {
	Id       int32  `json:"id"`
	Status   string `json:"status"`
	Amount   string `json:"amount"`
	VatValue string `json:"vat_value"`
	Total    string `json:"total"`
	Created  string `json:"created"`
}

type rawInvoiceList struct {
	Id             int32  `json:"id"`
	NumberProforma string `json:"number_proforma"`
	NumberFiscal   string `json:"number_fiscal"`
	Status         string `json:"status"`
	Subtotal       string `json:"subtotal"`
	VatValue       string `json:"vat_value"`
	Total          string `json:"total"`
	InvoiceDate    string `json:"invoice_date"`
	PaymentMethod  string `json:"payment_method"`
}

type rawServiceList struct {
	Id           int32  `json:"id"`
	Hostname     string `json:"hostname"`
	Status       string `json:"status"`
	Price        string `json:"price"`
	NextInvoice  string `json:"next_invoice"`
	BillingCycle string `json:"billing_cycle"`
	AutoPayment  string `json:"auto_payment"`
	Company      string `json:"company"`
}

type rawSubscription struct {
	Id              int32  `json:"id"`
	Status          string `json:"status"`
	ServiceHostname string `json:"service_hostname"`
	Subtotal        string `json:"subtotal"`
	VatValue        string `json:"vat_value"`
	Total           string `json:"total"`
	CreationDate    string `json:"creation_date"`
}

var (
	outputFormat = cmdutil.OutputFormat
	force        = cmdutil.Force
)

var Cmd = &cobra.Command{
	Use:     "billing",
	Aliases: []string{"bill"},
	Short:   "Manage billing, invoices, funds, and services",
	Args:    cobra.NoArgs,
}

// --- Funds / Balance ---

var fundsCmd = &cobra.Command{
	Use:   "funds",
	Short: "Manage account funds and balance",
	Args:  cobra.NoArgs,
}

var fundsBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show current account balance",
	RunE: func(cmd *cobra.Command, args []string) error {
		var balance rawFundsBalance
		if err := client.RawGet("/api/billing/funds/", &balance); err != nil {
			return fmt.Errorf("getting balance: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, balance, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "Balance:", balance.Balance)
			output.PrintRow(tw, "Threshold Type:", balance.ThresholdType)
			tw.Flush()
		})
	},
}

var fundsLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show funds activity log",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		logs, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.FundsLog, bool, error) {
			resp, _, err := c.BillingAPI.BillingFundsLogList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing funds log: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, logs, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "OPERATION", "AMOUNT", "BALANCE", "DATE")
			for _, l := range logs {
				output.PrintRow(tw, l.Id, l.Operation, l.Amount, l.Balance, l.Date)
			}
			tw.Flush()
		})
	},
}

// --- Deposits ---

var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Manage deposits",
	Args:  cobra.NoArgs,
}

var depositListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all deposits",
	RunE: func(cmd *cobra.Command, args []string) error {
		deposits, err := client.RawFetchAll[rawDeposit]("/api/billing/deposits/")
		if err != nil {
			return fmt.Errorf("listing deposits: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, deposits, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "AMOUNT", "TOTAL", "STATUS", "DATE")
			for _, d := range deposits {
				output.PrintRow(tw, d.Id, d.Amount, d.Total, d.Status, d.Created)
			}
			tw.Flush()
		})
	},
}

var depositGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get deposit details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var d rawDeposit
		if err := client.RawGet(fmt.Sprintf("/api/billing/deposits/%s/", args[0]), &d); err != nil {
			return fmt.Errorf("getting deposit: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, d, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", d.Id)
			output.PrintRow(tw, "Amount:", d.Amount)
			output.PrintRow(tw, "VAT:", d.VatValue)
			output.PrintRow(tw, "Total:", d.Total)
			output.PrintRow(tw, "Status:", d.Status)
			output.PrintRow(tw, "Created:", d.Created)
			tw.Flush()
		})
	},
}

var depositCreateAmount int32

var depositCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a deposit",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewDepositCreate(depositCreateAmount)
		resp, _, err := c.BillingAPI.BillingDepositsCreate(context.Background()).DepositCreate(body).Execute()
		if err != nil {
			return fmt.Errorf("creating deposit: %w", err)
		}
		fmt.Printf("Deposit created (ID: %d, Amount: %.2f)\n", resp.Id, resp.Amount)
		return nil
	},
}

// --- Invoices ---

var invoiceCmd = &cobra.Command{
	Use:   "invoice",
	Short: "Manage invoices",
	Args:  cobra.NoArgs,
}

var invoiceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all invoices",
	RunE: func(cmd *cobra.Command, args []string) error {
		invoices, err := client.RawFetchAll[rawInvoiceList]("/api/billing/invoices/")
		if err != nil {
			return fmt.Errorf("listing invoices: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, invoices, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "PROFORMA", "FISCAL", "TOTAL", "STATUS", "DATE")
			for _, inv := range invoices {
				output.PrintRow(tw, inv.Id, inv.NumberProforma, inv.NumberFiscal, inv.Total, inv.Status, inv.InvoiceDate)
			}
			tw.Flush()
		})
	},
}

var invoiceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get invoice details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var inv rawInvoiceList
		if err := client.RawGet(fmt.Sprintf("/api/billing/invoices/%s/", args[0]), &inv); err != nil {
			return fmt.Errorf("getting invoice: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, inv, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", inv.Id)
			output.PrintRow(tw, "Proforma:", inv.NumberProforma)
			output.PrintRow(tw, "Fiscal:", inv.NumberFiscal)
			output.PrintRow(tw, "Subtotal:", inv.Subtotal)
			output.PrintRow(tw, "VAT:", inv.VatValue)
			output.PrintRow(tw, "Total:", inv.Total)
			output.PrintRow(tw, "Status:", inv.Status)
			output.PrintRow(tw, "Date:", inv.InvoiceDate)
			output.PrintRow(tw, "Payment:", inv.PaymentMethod)
			tw.Flush()
		})
	},
}

var invoicePayCmd = &cobra.Command{
	Use:   "pay <id>",
	Short: "Pay an invoice using account funds",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Pay invoice %s using account funds?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingInvoicesPayWithFundsCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("paying invoice: %w", err)
		}
		fmt.Printf("Invoice paid: %s\n", resp.Message)
		return nil
	},
}

// --- Services ---

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage billing services",
	Args:  cobra.NoArgs,
}

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	RunE: func(cmd *cobra.Command, args []string) error {
		services, err := client.RawFetchAll[rawServiceList]("/api/billing/services/")
		if err != nil {
			return fmt.Errorf("listing services: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, services, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "STATUS", "PRICE", "CYCLE", "NEXT INVOICE")
			for _, s := range services {
				output.PrintRow(tw, s.Id, s.Hostname, s.Status, s.Price, s.BillingCycle, s.NextInvoice)
			}
			tw.Flush()
		})
	},
}

var serviceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get service details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var s rawServiceList
		if err := client.RawGet(fmt.Sprintf("/api/billing/services/%s/", args[0]), &s); err != nil {
			return fmt.Errorf("getting service: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.Hostname)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Price:", s.Price)
			output.PrintRow(tw, "Billing Cycle:", s.BillingCycle)
			output.PrintRow(tw, "Next Invoice:", s.NextInvoice)
			output.PrintRow(tw, "Auto Pay:", s.AutoPayment)
			output.PrintRow(tw, "Company:", s.Company)
			tw.Flush()
		})
	},
}

var serviceCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force(cmd) && !confirm.Action(fmt.Sprintf("Cancel service %s?", args[0])) {
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingServicesCancelCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("cancelling service: %w", err)
		}
		fmt.Printf("Service cancelled: %s\n", resp.Message)
		return nil
	},
}

var serviceAutoPayCmd = &cobra.Command{
	Use:   "toggle-auto-pay <id>",
	Short: "Toggle auto-payment for a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		resp, _, err := c.BillingAPI.BillingServicesToggleAutoPaymentCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("toggling auto-pay: %w", err)
		}
		state := "enabled"
		if !resp.AutoPayment {
			state = "disabled"
		}
		fmt.Printf("Auto-pay %s: %s\n", state, resp.Message)
		return nil
	},
}

// --- Subscriptions ---

var subscriptionCmd = &cobra.Command{
	Use:   "subscription",
	Short: "Manage subscriptions",
	Args:  cobra.NoArgs,
}

var subscriptionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all subscriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		subs, err := client.RawFetchAll[rawSubscription]("/api/billing/subscriptions/")
		if err != nil {
			return fmt.Errorf("listing subscriptions: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, subs, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "HOSTNAME", "STATUS", "TOTAL", "CREATED")
			for _, s := range subs {
				output.PrintRow(tw, s.Id, s.ServiceHostname, s.Status, s.Total, s.CreationDate)
			}
			tw.Flush()
		})
	},
}

var subscriptionGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get subscription details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var s rawSubscription
		if err := client.RawGet(fmt.Sprintf("/api/billing/subscriptions/%s/", args[0]), &s); err != nil {
			return fmt.Errorf("getting subscription: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, s, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", s.Id)
			output.PrintRow(tw, "Hostname:", s.ServiceHostname)
			output.PrintRow(tw, "Status:", s.Status)
			output.PrintRow(tw, "Subtotal:", s.Subtotal)
			output.PrintRow(tw, "VAT:", s.VatValue)
			output.PrintRow(tw, "Total:", s.Total)
			output.PrintRow(tw, "Created:", s.CreationDate)
			tw.Flush()
		})
	},
}

func init() {
	depositCreateCmd.Flags().Int32Var(&depositCreateAmount, "amount", 0, "Deposit amount in EUR (required)")
	depositCreateCmd.MarkFlagRequired("amount")

	fundsCmd.AddCommand(fundsBalanceCmd)
	fundsCmd.AddCommand(fundsLogCmd)

	depositCmd.AddCommand(depositListCmd)
	depositCmd.AddCommand(depositGetCmd)
	depositCmd.AddCommand(depositCreateCmd)

	invoiceCmd.AddCommand(invoiceListCmd)
	invoiceCmd.AddCommand(invoiceGetCmd)
	invoiceCmd.AddCommand(invoicePayCmd)

	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceGetCmd)
	serviceCmd.AddCommand(serviceCancelCmd)
	serviceCmd.AddCommand(serviceAutoPayCmd)

	subscriptionCmd.AddCommand(subscriptionListCmd)
	subscriptionCmd.AddCommand(subscriptionGetCmd)

	Cmd.AddCommand(fundsCmd)
	Cmd.AddCommand(depositCmd)
	Cmd.AddCommand(invoiceCmd)
	Cmd.AddCommand(serviceCmd)
	Cmd.AddCommand(subscriptionCmd)
}
