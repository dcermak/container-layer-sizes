package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrNonExistent = errors.New("No such entry found in the database")
)

type SQLiteBackend struct {
	con *sql.DB
}

func (b *SQLiteBackend) Migrate() error {
	query := `
    create table if not exists image_history_entry(
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        image_id INTEGER NOT NULL,
        hash TEXT NOT NULL,
        tags TEXT NOT NULL,
        contents TEXT NOT NULL,
        inspect_info TEXT NOT NULL,
        FOREIGN KEY(image_id) REFERENCES image(id)
    );
    CREATE TABLE IF NOT EXISTS image(
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL
    );
    `
	_, err := b.con.Exec(query)
	return err
}

func imageHistoryEntryToJson(imageHistoryEntry *ImageHistoryEntry) (tagsJson []byte, contentsJson []byte, inspectInfoJson []byte, err error) {
	tagsJson, err = json.Marshal(imageHistoryEntry.Tags)
	if err != nil {
		return nil, nil, nil, err
	}
	contentsJson, err = json.Marshal(imageHistoryEntry.Contents)
	if err != nil {
		return nil, nil, nil, err
	}
	inspectInfoJson, err = json.Marshal(imageHistoryEntry.InspectInfo)
	if err != nil {
		return nil, nil, nil, err
	}

	return
}

func (s *SQLiteBackend) createImageHistoryEntry(imageId int64, hash string, imageHistoryEntry *ImageHistoryEntry) (*ImageHistoryEntry, error) {
	tagsJson, contentsJson, inspectInfoJson, err := imageHistoryEntryToJson(imageHistoryEntry)
	if err != nil {
		return nil, err
	}
	res, err := s.con.Exec("INSERT INTO image_history_entry(image_id,hash,tags,contents,inspect_info) values(?,?,?,?,?)", imageId, hash, tagsJson, contentsJson, inspectInfoJson)

	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &ImageHistoryEntry{
		id:          id,
		Tags:        imageHistoryEntry.Tags,
		Contents:    imageHistoryEntry.Contents,
		InspectInfo: imageHistoryEntry.InspectInfo,
	}, nil
}

func (s *SQLiteBackend) updateImageHistoryEntry(imageId int64, hash string, imageHistoryEntry *ImageHistoryEntry) (*ImageHistoryEntry, error) {
	tagsJson, contentsJson, inspectInfoJson, err := imageHistoryEntryToJson(imageHistoryEntry)
	if err != nil {
		return nil, err
	}

	res, err := s.con.Exec("UPDATE image_history_entry SET image_id = ?, hash = ?, tags = ?, contents = ?, inspect_info = ? WHERE ID = ?", imageId, hash, tagsJson, contentsJson, inspectInfoJson, imageHistoryEntry.id)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, errors.New(fmt.Sprintf("Failed to update image row with id %d", imageHistoryEntry.id))
	}

	return imageHistoryEntry, nil
}

func (s *SQLiteBackend) getAllImageHistoryEntries(imageId int64) (map[string]ImageHistoryEntry, error) {
	rows, err := s.con.Query("SELECT * FROM image_history_entry WHERE image_id = ?", imageId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]ImageHistoryEntry)

	for rows.Next() {
		var entry ImageHistoryEntry
		var image_id int64
		var hash, tagsJson, contentsJson, inspectInfoJson string
		if err := rows.Scan(&entry.id, &image_id, &hash, &tagsJson, &contentsJson, &inspectInfoJson); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(tagsJson), &entry.Tags); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(contentsJson), &entry.Contents); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(inspectInfoJson), &entry.InspectInfo); err != nil {
			return nil, err
		}
		res[hash] = entry
	}

	return res, nil
}

func (s *SQLiteBackend) deleteTableById(tableName string, id int64) error {
	res, err := s.con.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), id)
	if err != nil {
		return err
	}
	if rows, err := res.RowsAffected(); err != nil {
		return err
	} else if rows == 0 {
		return errors.New(
			fmt.Sprintf("Delete of table %s with id %d failed: 0 rows deleted", tableName, id),
		)
	}
	return nil
}

func CreateSQLiteBackend(dbFileName string) (*SQLiteBackend, error) {
	con, err := sql.Open("sqlite3", dbFileName)
	if err != nil {
		return nil, err
	}
	backend := &SQLiteBackend{con: con}

	if err := backend.Migrate(); err != nil {
		return nil, err
	}
	return backend, nil
}

func (s *SQLiteBackend) Destroy() error {
	return s.con.Close()
}

func (s *SQLiteBackend) Create(imageHistory *ImageHistory) (*ImageHistory, error) {
	tx, err := s.con.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	res, err := s.con.Exec("INSERT INTO image(name) values(?)", imageHistory.Name)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	imageId, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	historyFromDb := make(map[string]ImageHistoryEntry, len(imageHistory.History))

	for hash, hist := range imageHistory.History {

		if newEntry, err := s.createImageHistoryEntry(imageId, hash, &hist); err != nil {
			tx.Rollback()
			return nil, err
		} else {
			historyFromDb[hash] = *newEntry
		}

	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return nil, err
	}

	imageHistory.History = historyFromDb
	imageHistory.ID = imageId
	return imageHistory, nil
}

func (s *SQLiteBackend) Delete(imageHistory *ImageHistory) error {
	tx, err := s.con.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	for _, hist := range imageHistory.History {
		if err := s.deleteTableById("image_history_entry", hist.id); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := s.deleteTableById("image", imageHistory.ID); err != nil {
		tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (s *SQLiteBackend) DeleteByName(imageName string) error {
	img, err := s.Read(imageName)
	if err != nil {
		return err
	}

	if imgLen := len(img); imgLen != 1 {
		return errors.New(
			fmt.Sprintf(
				"Expected to find exactly one image with the name %s, but got %d",
				imageName,
				imgLen,
			),
		)
	}

	return s.Delete(&img[0])
}

func (s *SQLiteBackend) Update(imageHistory *ImageHistory) (*ImageHistory, error) {
	tx, err := s.con.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	oldHistoryEntries, err := s.getAllImageHistoryEntries(imageHistory.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if res, err := s.con.Exec("UPDATE image SET name = ? WHERE ID = ?", imageHistory.Name, imageHistory.ID); err != nil {
		return nil, err
	} else if rowsAffected, e := res.RowsAffected(); e != nil {
		tx.Rollback()
		return nil, e
	} else if rowsAffected == 0 {
		tx.Rollback()
		return nil, errors.New(fmt.Sprintf("Failed to update image row with id %d", imageHistory.ID))
	}

	newHistory := make(map[string]ImageHistoryEntry)

	for hash, oldEntry := range oldHistoryEntries {
		if newEntry, ok := imageHistory.History[hash]; !ok {
			if err = s.deleteTableById("image_history_entry", oldEntry.id); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			updatedEntry, err := s.updateImageHistoryEntry(imageHistory.ID, hash, &newEntry)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			newHistory[hash] = *updatedEntry
		}
	}
	for hash, toCreateEntry := range imageHistory.History {
		if _, ok := oldHistoryEntries[hash]; !ok {
			newEntry, err := s.createImageHistoryEntry(imageHistory.ID, hash, &toCreateEntry)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			newHistory[hash] = *newEntry
		}
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return nil, err
	}
	imageHistory.History = newHistory
	return imageHistory, nil

}

func (s *SQLiteBackend) Read(imageName string) ([]ImageHistory, error) {
	rows, err := s.con.Query("SELECT * FROM image WHERE name = ?", imageName)
	if err != nil {
		return nil, err
	}

	res := make([]ImageHistory, 0)

	for rows.Next() {
		var entry ImageHistory
		if err = rows.Scan(&entry.ID, &entry.Name); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNonExistent
			}
			return nil, err
		}
		if historyEntry, err := s.getAllImageHistoryEntries(entry.ID); err != nil {
			return nil, err
		} else {
			entry.History = historyEntry
			res = append(res, entry)
		}

	}

	return res, nil
}

func (s *SQLiteBackend) ReadById(imageId int64) (*ImageHistory, error) {
	row := s.con.QueryRow("SELECT * FROM image where id = ?", imageId)

	var entry ImageHistory
	if err := row.Scan(&entry.ID, &entry.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNonExistent
		}
		return nil, err
	}
	if historyEntry, err := s.getAllImageHistoryEntries(entry.ID); err != nil {
		return nil, err
	} else {
		entry.History = historyEntry
		return &entry, nil
	}
}
