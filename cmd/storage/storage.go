// Package storage holds the object-storage commands.
//
// Buckets live under the `cloud` API tag alongside compute, but they get their
// own top-level group: `compute volume` is block storage attached to a server,
// a bucket is a standalone object store, and sitting the two next to each other
// invites picking the wrong one. The group name describes the product an
// operator buys rather than the tag the schema files it under.
package storage

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage object storage",
	Args:  cobra.NoArgs,
}

func init() {
	Cmd.AddCommand(bucketCmd)
}
