package db

func (d *DB) AddDid(did string) error {
	_, err := d.db.Exec(`insert or ignore into known_dids (did) values (?)`, did)
	return err
}

func (d *DB) RemoveDid(did string) error {
	_, err := d.db.Exec(`delete from known_dids where did = ?`, did)
	return err
}

func (d *DB) GetAllDids() ([]string, error) {
	var dids []string

	rows, err := d.db.Query(`select did from known_dids`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, err
		}
		dids = append(dids, did)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dids, nil
}

func (d *DB) HasKnownDids() bool {
	var count int
	err := d.db.QueryRow(`select count(*) from known_dids`).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}
