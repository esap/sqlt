package sqlt

import (
	"bytes"
	"context"
	"io"
	"log"
	"text/template"
	"time"

	//	log "github.com/it512/slf4go"
)

var Debug = false

type (
	SqlRender interface {
		Render(w io.Writer, id string, param interface{}) error
	}

	StdTemplateRender struct {
		pattern string
		funcMap template.FuncMap
		t       *template.Template
	}
)

func (st *StdTemplateRender) Render(w io.Writer, id string, param interface{}) error {
	return st.t.ExecuteTemplate(w, id, param)
}

func NewStdTemplateRender(pattern string, funcMap template.FuncMap) *StdTemplateRender {
	tpl := template.New("sqlt-std-template").Funcs(funcMap)
	tpl = template.Must(tpl.ParseGlob(pattern))
	return &StdTemplateRender{pattern: pattern, funcMap: funcMap, t: tpl}
}

func NewStdTemplateRenderDefault(pattern string) *StdTemplateRender {
	return NewStdTemplateRender(pattern, make(template.FuncMap))
}

type (
	Config struct {
		TimeOut  int64
		ReadOnly bool
		Extra    map[string]interface{}
	}

	Manifest struct {
		Default   Config
		ConfigMap map[string]Config
	}
)

var (
	DefaultManifest = Manifest{
		Default: Config{
			TimeOut:  0,
			ReadOnly: false,
			Extra:    make(map[string]interface{}),
		},
		ConfigMap: make(map[string]Config),
	}
)

func (m Manifest) GetConfigCopy(id string) Config {
	if c, ok := m.ConfigMap[id]; ok {
		config := Config{TimeOut: c.TimeOut, ReadOnly: c.ReadOnly, Extra: c.Extra}
		if config.TimeOut == 0 {
			config.TimeOut = m.Default.TimeOut
		}

		return config
	}

	return m.Default
}

type (
	StdSqlDescriber struct {
		Id     string
		Data   interface{}
		Config Config
		bytes.Buffer
		cf context.CancelFunc
	}
)

func (s *StdSqlDescriber) GetSql(c context.Context) (string, context.Context, error) {
	return s.String(), s.WithContext(c), nil
}

func (s *StdSqlDescriber) WithContext(c context.Context) context.Context {
	if s.Config.TimeOut > 0 {
		ctx, cf := context.WithTimeout(c, time.Duration(s.Config.TimeOut)*time.Millisecond)
		s.cf = cf
		return ctx
	}
	return c
}

func (s *StdSqlDescriber) Release() {
	if s.cf != nil {
		s.cf()
	}
}

func (s *StdSqlDescriber) IsReadOnly() bool {
	return s.Config.ReadOnly
}

type (
	StdSqlAssembler struct {
		Render   SqlRender
		Manifest Manifest
	}
)

func (l *StdSqlAssembler) AssembleSql(id string, data interface{}) (SqlDescriber, error) {
	desc := new(StdSqlDescriber)
	desc.Id = id
	desc.Data = data
	desc.Config = l.Manifest.GetConfigCopy(id)

	e := l.Render.Render(desc, id, data)

	if e == nil && Debug {
		log.Println(desc, data)
	}

	return desc, e
}

func NewStdSqlAssemblerDefault(pattern string) *StdSqlAssembler {
	return NewStdSqlAssembler(NewStdTemplateRenderDefault(pattern), DefaultManifest)
}

func NewStdSqlAssembler(r SqlRender, m Manifest) *StdSqlAssembler {
	return &StdSqlAssembler{Render: r, Manifest: m}
}

type (
	NestableSqlAssmbler interface {
		SqlAssembler
		HasId(id string) bool
	}

	SqlAssemblerSet struct {
		def        SqlAssembler
		assemblers []NestableSqlAssmbler
	}
)

func (n *SqlAssemblerSet) AssembleSql(id string, data interface{}) (SqlDescriber, error) {
	for _, l := range n.assemblers {
		if l.HasId(id) {
			return l.AssembleSql(id, data)
		}
	}

	return n.def.AssembleSql(id, data)
}

func NewSqlAssemblerSet(def SqlAssembler, assemblers ...NestableSqlAssmbler) *SqlAssemblerSet {
	return &SqlAssemblerSet{def: def, assemblers: assemblers}
}
