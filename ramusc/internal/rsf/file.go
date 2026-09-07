package rsf

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// File — файл .rsf целиком: разобранные таблицы и всё остальное как есть.
//
// Ramus при сохранении переписывает всё, что начинается с data/, а прочие
// записи копирует из старого файла (RSF-FORMAT.md §1). Здесь так же: Raw
// переносится побайтово, порядок записей сохраняется.
type File struct {
	Order  []string          // имена записей ZIP в исходном порядке
	Tables map[string]*Table // путь внутри ZIP → таблица
	Raw    map[string][]byte // записи, которые таблицами не являются

	// GenerateTime — отметка из атрибута generate-time. В файле она одна
	// на все таблицы, поэтому и здесь хранится один раз.
	GenerateTime string
}

// Open читает файл с диска.
func Open(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return Read(f, info.Size())
}

// Read читает .rsf из произвольного источника.
func Read(r io.ReaderAt, size int64) (*File, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("не похоже на .rsf (ZIP не открывается): %w", err)
	}

	file := &File{
		Tables: make(map[string]*Table),
		Raw:    make(map[string][]byte),
	}
	for _, entry := range zr.File {
		data, err := readEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("запись %s: %w", entry.Name, err)
		}
		file.Order = append(file.Order, entry.Name)

		table, generateTime, ok := parseEntry(entry.Name, data)
		if !ok {
			file.Raw[entry.Name] = data
			continue
		}
		file.Tables[entry.Name] = table
		if file.GenerateTime == "" {
			file.GenerateTime = generateTime
		}
	}
	return file, nil
}

// parseEntry решает, таблица перед нами или нет. Таблицы лежат только под
// data/; всё прочее — потоки пользователя, их разбирать нечем и незачем.
func parseEntry(name string, data []byte) (*Table, string, bool) {
	if !strings.HasPrefix(name, "data/") || !strings.HasSuffix(name, ".xml") {
		return nil, "", false
	}
	return ParseTable(name, data)
}

func readEntry(entry *zip.File) ([]byte, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Table находит таблицу по короткому имени (elements), по имени с плагином
// (IDEF0/attribute_rectangles) или по полному пути внутри ZIP.
//
// В отличие от эталонной реализации на Python неоднозначность — ошибка:
// молча взять первую попавшуюся значит однажды долго искать, почему правка
// ушла не в ту таблицу.
func (f *File) Table(name string) (*Table, error) {
	var found []*Table
	for path, t := range f.Tables {
		if path == name || path == "data/"+name+".xml" || t.Name == name {
			found = append(found, t)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return nil, fmt.Errorf("таблица %s не найдена", name)
	default:
		paths := make([]string, 0, len(found))
		for _, t := range found {
			paths = append(paths, t.Path)
		}
		sort.Strings(paths)
		return nil, fmt.Errorf("имя %s неоднозначно: %s", name, strings.Join(paths, ", "))
	}
}

// MustTable удобен там, где отсутствие таблицы означает не тот файл, а не
// ошибку пользователя.
func (f *File) MustTable(name string) *Table {
	t, err := f.Table(name)
	if err != nil {
		panic(err)
	}
	return t
}

// NextID отдаёт MAX(column)+1. Для большинства последовательностей этого
// достаточно: Ramus перематывает их сам при коллизии (§1 формата).
func (f *File) NextID(table, column string) (int64, error) {
	t, err := f.Table(table)
	if err != nil {
		return 0, err
	}
	var max int64
	for _, row := range t.Rows {
		v := t.Str(row, column)
		if v == "" {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// Encode собирает ZIP заново: таблицы сериализуются, прочее переносится как
// есть, порядок записей сохраняется.
func (f *File) Encode(w io.Writer) error {
	zw := zip.NewWriter(w)
	for _, name := range f.Order {
		var payload []byte
		if t, ok := f.Tables[name]; ok {
			payload = t.Encode(f.GenerateTime)
		} else {
			payload = f.Raw[name]
		}

		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		})
		if err != nil {
			return err
		}
		if _, err := entry.Write(payload); err != nil {
			return err
		}
	}
	return zw.Close()
}

// Bytes собирает файл в память.
func (f *File) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := f.Encode(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save записывает файл на диск.
func (f *File) Save(path string) error {
	data, err := f.Bytes()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
