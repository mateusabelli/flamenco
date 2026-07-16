package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/pkg/crosspath"
)

func init() {
	goose.AddMigrationContext(upReconstructShamanCheckoutID, downReconstructShamanCheckoutID)
}

// jobsPrefix is the variable that's placed in front of blend file paths when they've been uploaded
// with the Shaman system.
const jobsPrefix = "{jobs}/"

// upReconstructShamanCheckoutID reconstructs the Shaman Checkout ID from the `blendfile` field in
// the job settings.
//
// If the blendfile path starts with `{jobs}/`, it's assumed to be a Shaman job.
func upReconstructShamanCheckoutID(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, uuid, name, settings ->> 'blendfile'
		FROM jobs
		WHERE settings ->> 'blendfile' IS NOT NULL
		  AND (storage_shaman_checkout_id IS NULL OR storage_shaman_checkout_id = '')
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type update struct {
		id         int64
		checkoutID string
		// Just for logging:
		uuid      string
		name      string
		blendfile string
	}
	// updates contains the job ID and the reconstructed checkout ID. These are gathered here, to
	// avoid executing SQL updates while looping over the results of another SQL query.
	var updates []update

	for rows.Next() {
		var (
			id        int64
			uuid      string
			name      string
			blendfile string
		)
		if err := rows.Scan(&id, &uuid, &name, &blendfile); err != nil {
			return err
		}

		// The code below turns a blendfile path like "{jobs}/checkout-id/subdir/file.blend" into the
		// "checkout-id" part. It also takes into account that the "checkout-id" can contain slashes
		// itself.

		blendfile = crosspath.ToSlash(blendfile)
		if !strings.HasPrefix(blendfile, jobsPrefix) {
			continue
		}
		tail := strings.TrimPrefix(blendfile, jobsPrefix)

		// Match a name "a/b/c" to a blendfile "{jobs}/a/b/c-suffix/d/e.blend".
		numSegmentsInName := strings.Count(crosspath.ToSlash(name), "/") + 1

		parts := strings.SplitN(tail, "/", numSegmentsInName+1)
		if len(parts) < numSegmentsInName {
			log.Warn().
				Str("jobID", uuid).
				Str("name", name).
				Str("blendfile", blendfile).
				Msg("migrating database, found unexpectedly short blendfile path given the job name")
			continue
		}
		updates = append(updates, update{
			id:         id,
			checkoutID: strings.Join(parts[:numSegmentsInName], "/"),

			uuid:      uuid,
			name:      name,
			blendfile: blendfile,
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(updates) == 0 {
		return nil
	}

	log.Info().
		Int("numJobs", len(updates)).
		Msg("migrating database, correcting Shaman checkout ID on jobs")

	stmt, err := tx.PrepareContext(ctx,
		`UPDATE jobs SET storage_shaman_checkout_id = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, u := range updates {
		log.Info().
			Str("jobID", u.uuid).
			Str("name", u.name).
			Str("blendfile", u.blendfile).
			Str("checkoutID", u.checkoutID).
			Msg("  setting Shaman checkout ID on job")
		if _, err := stmt.ExecContext(ctx, u.checkoutID, u.id); err != nil {
			return err
		}
	}
	return nil
}

func downReconstructShamanCheckoutID(ctx context.Context, tx *sql.Tx) error {
	// No-op: can't distinguish reconstructed values from legitimately-set ones. And there is no
	// reason to erase the information that was missing because of a bug.
	return nil
}
