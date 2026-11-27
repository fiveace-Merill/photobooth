package admin

import (
	"encoding/csv"
	"io"
)

func neutralizeCSVCell(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	default:
		return v
	}
}

func writeWaitlistCSV(w io.Writer, rows []WaitlistExportRow) error {
	cw := csv.NewWriter(w)
	header := []string{"id", "name", "contact_email", "contact_phone", "photography_type", "shoots_per_month", "consent_at", "consent_version", "status", "invitation_id", "invited_at", "notes", "created_at", "updated_at"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		rec := []string{
			neutralizeCSVCell(r.ID), neutralizeCSVCell(r.Name), neutralizeCSVCell(maskEmail(r.ContactEmail)), neutralizeCSVCell(maskPhone(r.ContactPhone)),
			neutralizeCSVCell(r.PhotographyType), neutralizeCSVCell(r.ShootsPerMonth), neutralizeCSVCell(r.ConsentAt), neutralizeCSVCell(r.ConsentVersion),
			neutralizeCSVCell(r.Status), neutralizeCSVCell(r.InvitationID), neutralizeCSVCell(r.InvitedAt), neutralizeCSVCell(r.Notes), neutralizeCSVCell(r.CreatedAt), neutralizeCSVCell(r.UpdatedAt),
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

type WaitlistExportRow struct {
	ID, Name, ContactEmail, ContactPhone, PhotographyType, ShootsPerMonth, ConsentAt, ConsentVersion, Status, InvitationID, InvitedAt, Notes, CreatedAt, UpdatedAt string
}
