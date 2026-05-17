package postgres

// scanner abstracts pgx.Row and pgx.Rows so scan helpers work for both.
type scanner interface {
	Scan(dest ...any) error
}
