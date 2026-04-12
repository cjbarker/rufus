package db

import (
	"database/sql"
	"fmt"
)

// InsertFace inserts a face detection record and returns its ID.
func (s *Store) InsertFace(rec *FaceRecord) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO faces (image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ImageID, rec.Descriptor, rec.BoundsX, rec.BoundsY, rec.BoundsW, rec.BoundsH, rec.PersonID,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting face: %w", err)
	}
	return result.LastInsertId()
}

// DeleteFacesByImage removes all face records for a given image. Used when
// force-rescanning to prevent accumulating duplicate detections.
func (s *Store) DeleteFacesByImage(imageID int64) error {
	_, err := s.db.Exec("DELETE FROM faces WHERE image_id = ?", imageID)
	if err != nil {
		return fmt.Errorf("deleting faces for image %d: %w", imageID, err)
	}
	return nil
}

// GetAllFacesWithPerson returns all face records joined with their person name
// (empty string when unlabeled).
func (s *Store) GetAllFacesWithPerson() ([]FaceWithPerson, error) {
	rows, err := s.db.Query(`
		SELECT f.id, f.image_id, f.descriptor, f.bounds_x, f.bounds_y, f.bounds_w, f.bounds_h,
		       f.person_id, COALESCE(p.name, '') as person_name
		FROM faces f
		LEFT JOIN people p ON p.id = f.person_id
		ORDER BY f.id`)
	if err != nil {
		return nil, fmt.Errorf("querying faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []FaceWithPerson
	for rows.Next() {
		var fw FaceWithPerson
		if err := rows.Scan(&fw.Face.ID, &fw.Face.ImageID, &fw.Face.Descriptor,
			&fw.Face.BoundsX, &fw.Face.BoundsY, &fw.Face.BoundsW, &fw.Face.BoundsH,
			&fw.Face.PersonID, &fw.PersonName); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		results = append(results, fw)
	}
	return results, rows.Err()
}

// GetUnlabeledFaces returns detected faces with no person assignment, joined
// with the image file path for display purposes.
func (s *Store) GetUnlabeledFaces() ([]UnlabeledFace, error) {
	rows, err := s.db.Query(`
		SELECT f.id, i.file_path, f.bounds_x, f.bounds_y, f.bounds_w, f.bounds_h
		FROM faces f
		JOIN images i ON i.id = f.image_id
		WHERE f.person_id IS NULL
		ORDER BY i.file_path, f.id`)
	if err != nil {
		return nil, fmt.Errorf("querying unlabeled faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var faces []UnlabeledFace
	for rows.Next() {
		var f UnlabeledFace
		if err := rows.Scan(&f.FaceID, &f.FilePath, &f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH); err != nil {
			return nil, fmt.Errorf("scanning unlabeled face row: %w", err)
		}
		faces = append(faces, f)
	}
	return faces, rows.Err()
}

// GetUnlabeledFacesWithDescriptors returns unlabeled face records including
// their descriptor blobs for re-matching against known labels.
func (s *Store) GetUnlabeledFacesWithDescriptors() ([]FaceRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id
		FROM faces WHERE person_id IS NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("querying unlabeled faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var recs []FaceRecord
	for rows.Next() {
		var f FaceRecord
		if err := rows.Scan(&f.ID, &f.ImageID, &f.Descriptor,
			&f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH, &f.PersonID); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		recs = append(recs, f)
	}
	return recs, rows.Err()
}

// WalkUnlabeledFacesWithDescriptors streams face records that have no assigned
// person, invoking fn for each row without loading the full result set into
// memory. WAL mode allows fn to safely call other Store methods. Return a
// non-nil error from fn to abort the walk.
func (s *Store) WalkUnlabeledFacesWithDescriptors(fn func(FaceRecord) error) error {
	rows, err := s.db.Query(`
		SELECT id, image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id
		FROM faces WHERE person_id IS NULL ORDER BY id`)
	if err != nil {
		return fmt.Errorf("querying unlabeled faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var f FaceRecord
		if err := rows.Scan(&f.ID, &f.ImageID, &f.Descriptor,
			&f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH, &f.PersonID); err != nil {
			return fmt.Errorf("scanning face row: %w", err)
		}
		if err := fn(f); err != nil {
			return err
		}
	}
	return rows.Err()
}

// GetFacesByImage returns all face records for a given image.
func (s *Store) GetFacesByImage(imageID int64) ([]FaceRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id
		FROM faces WHERE image_id = ?`, imageID)
	if err != nil {
		return nil, fmt.Errorf("querying faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var faces []FaceRecord
	for rows.Next() {
		var f FaceRecord
		if err := rows.Scan(&f.ID, &f.ImageID, &f.Descriptor,
			&f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH, &f.PersonID); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		faces = append(faces, f)
	}
	return faces, rows.Err()
}

// GetFacesWithPersonByImage returns all face records for the given image,
// joined with their assigned person name (empty string when unlabeled).
func (s *Store) GetFacesWithPersonByImage(imageID int64) ([]FaceWithPerson, error) {
	rows, err := s.db.Query(`
		SELECT f.id, f.image_id, f.descriptor, f.bounds_x, f.bounds_y, f.bounds_w, f.bounds_h,
		       f.person_id, COALESCE(p.name, '') as person_name
		FROM faces f
		LEFT JOIN people p ON p.id = f.person_id
		WHERE f.image_id = ?
		ORDER BY f.id`, imageID)
	if err != nil {
		return nil, fmt.Errorf("querying faces for image %d: %w", imageID, err)
	}
	defer func() { _ = rows.Close() }()

	var results []FaceWithPerson
	for rows.Next() {
		var fw FaceWithPerson
		if err := rows.Scan(&fw.Face.ID, &fw.Face.ImageID, &fw.Face.Descriptor,
			&fw.Face.BoundsX, &fw.Face.BoundsY, &fw.Face.BoundsW, &fw.Face.BoundsH,
			&fw.Face.PersonID, &fw.PersonName); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		results = append(results, fw)
	}
	return results, rows.Err()
}

// UpdateFacePerson assigns a person ID to a face.
func (s *Store) UpdateFacePerson(faceID, personID int64) error {
	_, err := s.db.Exec("UPDATE faces SET person_id = ? WHERE id = ?", personID, faceID)
	return err
}

// UnlabelFace removes the person assignment from a face (sets person_id to NULL).
func (s *Store) UnlabelFace(faceID int64) error {
	_, err := s.db.Exec("UPDATE faces SET person_id = NULL WHERE id = ?", faceID)
	if err != nil {
		return fmt.Errorf("unlabeling face %d: %w", faceID, err)
	}
	return nil
}

// InsertPerson inserts a named person and returns the new person ID.
func (s *Store) InsertPerson(name string) (int64, error) {
	result, err := s.db.Exec("INSERT INTO people (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("inserting person: %w", err)
	}
	return result.LastInsertId()
}

// GetPersonByName returns a person by name, or nil if not found.
func (s *Store) GetPersonByName(name string) (*PersonRecord, error) {
	var p PersonRecord
	err := s.db.QueryRow("SELECT id, name, created_at FROM people WHERE name = ?", name).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying person: %w", err)
	}
	return &p, nil
}

// GetAllPeople returns all named people ordered by name.
func (s *Store) GetAllPeople() ([]PersonRecord, error) {
	rows, err := s.db.Query("SELECT id, name, created_at FROM people ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("querying people: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var people []PersonRecord
	for rows.Next() {
		var p PersonRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning person row: %w", err)
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

// MergePeople reassigns all faces from mergeID to keepID, then deletes mergeID.
// Both operations run inside a single transaction.
func (s *Store) MergePeople(keepID, mergeID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning merge transaction: %w", err)
	}
	if _, err := tx.Exec("UPDATE faces SET person_id = ? WHERE person_id = ?", keepID, mergeID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("reassigning faces: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM people WHERE id = ?", mergeID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting merged person: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing merge: %w", err)
	}
	return nil
}
