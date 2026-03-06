package support

import (
	"context"
	"fmt"
	"io"

	pidginhost "github.com/pidginhost/sdk-go"
	"github.com/spf13/cobra"

	"github.com/pidginhost/phctl/internal/client"
	"github.com/pidginhost/phctl/internal/cmdutil"
	"github.com/pidginhost/phctl/internal/output"
)

var (
	outputFormat = cmdutil.OutputFormat
)

var Cmd = &cobra.Command{
	Use:     "support",
	Aliases: []string{"ticket"},
	Short:   "Manage support tickets",
}

// --- Departments ---

var departmentCmd = &cobra.Command{
	Use:   "department",
	Short: "List support departments",
}

var departmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all support departments",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		depts, _, err := c.SupportAPI.SupportDepartmentsList(context.Background()).Execute()
		if err != nil {
			return fmt.Errorf("listing departments: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, depts, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "TITLE")
			for _, d := range depts {
				output.PrintRow(tw, d.Id, d.Title)
			}
			tw.Flush()
		})
	},
}

// --- Tickets ---

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage support tickets",
}

var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tickets",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		tickets, err := cmdutil.FetchAll(func(page int32) ([]pidginhost.TicketList, bool, error) {
			resp, _, err := c.SupportAPI.SupportTicketsList(context.Background()).Page(page).Execute()
			if err != nil {
				return nil, false, err
			}
			return resp.Results, resp.Next.Get() != nil, nil
		})
		if err != nil {
			return fmt.Errorf("listing tickets: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, tickets, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID", "SUBJECT", "DEPARTMENT", "STATUS", "CREATED")
			for _, t := range tickets {
				output.PrintRow(tw, t.Id, t.Subject, t.Department, t.Status, t.Created)
			}
			tw.Flush()
		})
	},
}

var ticketGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get ticket details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		t, _, err := c.SupportAPI.SupportTicketsRetrieve(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("getting ticket: %w", err)
		}
		format := outputFormat(cmd)
		return output.Print(format, t, func(w io.Writer) {
			tw := output.NewTabWriter(w)
			output.PrintRow(tw, "ID:", t.Id)
			output.PrintRow(tw, "Subject:", t.Subject)
			output.PrintRow(tw, "Department:", t.Department.Title)
			output.PrintRow(tw, "Priority:", t.Priority)
			output.PrintRow(tw, "Status:", t.Status)
			output.PrintRow(tw, "Created:", t.Created)
			output.PrintRow(tw, "Updated:", t.Updated)
			tw.Flush()
			if t.Messages != "" {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Messages:")
				fmt.Fprintln(w, t.Messages)
			}
		})
	},
}

var (
	ticketCreateSubject string
	ticketCreateDept    int32
	ticketCreateMessage string
)

var ticketCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a support ticket",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewTicketCreate(ticketCreateSubject, ticketCreateDept, ticketCreateMessage)
		resp, _, err := c.SupportAPI.SupportTicketsCreate(context.Background()).TicketCreate(body).Execute()
		if err != nil {
			return fmt.Errorf("creating ticket: %w", err)
		}
		fmt.Printf("Ticket created (ID: %d, Subject: %s)\n", resp.Id, resp.Subject)
		return nil
	},
}

var ticketReplyMessage string

var ticketReplyCmd = &cobra.Command{
	Use:   "reply <id>",
	Short: "Reply to a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		body := *pidginhost.NewTicketReply(ticketReplyMessage)
		_, _, err = c.SupportAPI.SupportTicketsReplyCreate(context.Background(), args[0]).TicketReply(body).Execute()
		if err != nil {
			return fmt.Errorf("replying to ticket: %w", err)
		}
		fmt.Printf("Reply sent to ticket %s.\n", args[0])
		return nil
	},
}

var ticketCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.SupportAPI.SupportTicketsCloseCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("closing ticket: %w", err)
		}
		fmt.Printf("Ticket %s closed.\n", args[0])
		return nil
	},
}

var ticketReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New()
		if err != nil {
			return err
		}
		_, _, err = c.SupportAPI.SupportTicketsReopenCreate(context.Background(), args[0]).Execute()
		if err != nil {
			return fmt.Errorf("reopening ticket: %w", err)
		}
		fmt.Printf("Ticket %s reopened.\n", args[0])
		return nil
	},
}

func init() {
	ticketCreateCmd.Flags().StringVar(&ticketCreateSubject, "subject", "", "Ticket subject (required)")
	ticketCreateCmd.Flags().Int32Var(&ticketCreateDept, "department", 0, "Department ID (required)")
	ticketCreateCmd.Flags().StringVar(&ticketCreateMessage, "message", "", "Initial message (required)")
	ticketCreateCmd.MarkFlagRequired("subject")
	ticketCreateCmd.MarkFlagRequired("department")
	ticketCreateCmd.MarkFlagRequired("message")

	ticketReplyCmd.Flags().StringVar(&ticketReplyMessage, "message", "", "Reply message (required)")
	ticketReplyCmd.MarkFlagRequired("message")

	departmentCmd.AddCommand(departmentListCmd)

	ticketCmd.AddCommand(ticketListCmd)
	ticketCmd.AddCommand(ticketGetCmd)
	ticketCmd.AddCommand(ticketCreateCmd)
	ticketCmd.AddCommand(ticketReplyCmd)
	ticketCmd.AddCommand(ticketCloseCmd)
	ticketCmd.AddCommand(ticketReopenCmd)

	Cmd.AddCommand(departmentCmd)
	Cmd.AddCommand(ticketCmd)
}
