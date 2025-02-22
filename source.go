package migrate

import (
	"embed"
	"path"
)

type Source interface {
	List() ([]string, error)
}

type StringMigrations []string

func (s StringMigrations) List() ([]string, error) {
	return s, nil
}

type EmbeddedMigrations struct {
	FS   embed.FS
	Path string
}

func (e EmbeddedMigrations) List() ([]string, error) {
	files, err := e.FS.ReadDir(e.Path)
	if err != nil {
		return nil, errf("reading embedded migration directory: %v", err)
	}

	ss := make([]string, 0, len(files))
	for _, f := range files {
		if f.Type().IsDir() {
			continue
		}

		p := path.Join(e.Path, f.Name())
		s, err := e.FS.ReadFile(p)
		if err != nil {
			return nil, errf("reading embedded migration file: %v", err)
		}

		ss = append(ss, string(s))
	}

	return ss, nil
}
