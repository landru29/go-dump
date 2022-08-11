package dump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// Encoder ensures all options to dump an object
type Encoder struct {
	Formatters  []KeyFormatterFunc
	ExtraFields struct {
		Len            bool
		Type           bool
		DetailedStruct bool
		DetailedMap    bool
		DetailedArray  bool
		DeepJSON       bool
		UseJSONTag     bool
	}
	ArrayJSONNotation bool
	Separator         string
	DisableTypePrefix bool
	Prefix            string
	writer            io.Writer
}

// NewDefaultEncoder instanciate a go-dump encoder
func NewDefaultEncoder() *Encoder {
	return NewEncoder(new(bytes.Buffer))
}

// NewEncoder instanciate a go-dump encoder over the writer
func NewEncoder(w io.Writer) *Encoder {
	enc := &Encoder{
		Formatters: []KeyFormatterFunc{
			WithDefaultFormatter(),
		},
		Separator: ".",
		writer:    w,
	}
	return enc
}

// Fdump formats and displays the passed arguments to io.Writer w. It formats exactly the same as Dump.
func (e *Encoder) Fdump(i interface{}) (err error) {
	res, err := e.ToStringMap(i)
	if err != nil {
		return
	}

	keys := []string{}
	for k := range res {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var err error
		if res[k] == "" {
			_, err = fmt.Fprintf(e.writer, "%s:\n", k)
		} else {
			_, err = fmt.Fprintf(e.writer, "%s: %s\n", k, res[k])
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Sdump returns a string with the passed arguments formatted exactly the same as Dump.
func (e *Encoder) Sdump(i interface{}) (string, error) {
	m, err := e.ToStringMap(i)
	if err != nil {
		return "", err
	}
	res := ""
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		res += fmt.Sprintf("%s: %s\n", k, m[k])
	}
	return res, nil
}

func (e *Encoder) fdumpInterface(w map[string]interface{}, i interface{}, roots []string) error {
	f := valueFromInterface(i)
	k := reflect.ValueOf(i).Kind()
	if k == reflect.Ptr && reflect.ValueOf(i).IsNil() || !validAndNotEmpty(f) {
		if len(roots) == 0 {
			return nil
		}
		k := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
		var prefix string
		if e.Prefix != "" {
			prefix = e.Prefix + e.Separator
		}
		w[prefix+k] = ""
		return nil
	}
	switch f.Kind() {
	case reflect.Struct:
		if e.ExtraFields.Type {
			nodeType := append(roots, "__Type__")
			nodeTypeFormatted := strings.Join(sliceFormat(nodeType, e.Formatters), e.Separator)
			w[nodeTypeFormatted] = f.Type().Name()
		}
		croots := roots
		if len(roots) == 0 && !e.DisableTypePrefix {
			croots = append(roots, f.Type().Name())
		}
		if err := e.fdumpStruct(w, f, croots); err != nil {
			return err
		}
	case reflect.Array, reflect.Slice:
		if err := e.fDumpArray(w, i, roots); err != nil {
			return err
		}
		return nil
	case reflect.Map:
		if e.ExtraFields.Type {
			nodeType := append(roots, "__Type__")
			nodeTypeFormatted := strings.Join(sliceFormat(nodeType, e.Formatters), e.Separator)
			w[nodeTypeFormatted] = "Map"
		}
		if err := e.fDumpMap(w, i, roots); err != nil {
			return err
		}
		return nil
	default:
		k := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
		if e.ExtraFields.DeepJSON && (f.Kind() == reflect.String) {
			if err := e.fDumpJSON(w, f.Interface().(string), roots, k); err != nil {
				return err
			}
		} else {
			var prefix string
			if e.Prefix != "" {
				prefix = e.Prefix + e.Separator
			}
			w[prefix+k] = f.Interface()
		}

	}
	return nil
}

func (e *Encoder) fDumpJSON(w map[string]interface{}, i string, roots []string, k string) error {
	var value interface{}
	bodyJSONArray := []interface{}{}
	// Try to parse as a json array
	if err := json.Unmarshal([]byte(i), &bodyJSONArray); err != nil {
		//Try to parse as a map
		bodyJSONMap := map[string]interface{}{}
		if err2 := json.Unmarshal([]byte(i), &bodyJSONMap); err2 == nil {
			value = bodyJSONMap
		} else {
			value = i
		}
	} else {
		value = bodyJSONArray
	}

	if value == i {
		var prefix string
		if e.Prefix != "" {
			prefix = e.Prefix + e.Separator
		}
		w[prefix+k] = i
		return nil
	}
	if err := e.fdumpInterface(w, value, roots); err != nil {
		return err
	}
	return nil
}

func (e *Encoder) fDumpArray(w map[string]interface{}, i interface{}, roots []string) error {
	f := valueFromInterface(i)
	if _, ok := f.Interface().([]byte); ok {
		if err := e.fdumpInterface(w, string(f.Interface().([]byte)), roots); err != nil {
			return err
		}
		return nil
	}

	if e.ExtraFields.Type {
		nodeType := append(roots, "__Type__")
		nodeTypeFormatted := strings.Join(sliceFormat(nodeType, e.Formatters), e.Separator)
		w[nodeTypeFormatted] = "Array"
	}

	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if e.ExtraFields.Len {
		nodeLen := append(roots, "__Len__")
		nodeLenFormatted := strings.Join(sliceFormat(nodeLen, e.Formatters), e.Separator)
		w[nodeLenFormatted] = v.Len()
	}

	if e.ExtraFields.DetailedArray && len(roots) > 0 {
		structKey := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
		w[structKey] = i
	}

	for i := 0; i < v.Len(); i++ {
		var l string
		var croots []string
		if len(roots) > 0 {
			l = roots[len(roots)-1:][0]
			if !e.ArrayJSONNotation {
				croots = append(roots, fmt.Sprintf("%s%d", l, i))
			} else {
				var t = make([]string, len(roots)-1)
				copy(t, roots[0:len(roots)-1])
				croots = append(t, fmt.Sprintf("%s[%d]", l, i))
			}
		} else {
			var skey = fmt.Sprintf("[%d]", i)
			if !e.ArrayJSONNotation {
				skey = fmt.Sprintf("%s%d", e.Prefix+l, i)
			}
			croots = append(roots, skey)
		}
		f := v.Index(i)

		stringer, ok := f.Interface().(fmt.Stringer)
		if ok {
			k := strings.Join(sliceFormat(croots, e.Formatters), e.Separator)
			var prefix string
			if e.Prefix != "" {
				prefix = e.Prefix
			}
			w[prefix+k] = stringer.String()
		}

		if err := e.fdumpInterface(w, f.Interface(), croots); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) fDumpMap(w map[string]interface{}, i interface{}, roots []string) error {
	v := reflect.ValueOf(i)

	keys := v.MapKeys()
	var lenKeys int64
	for _, k := range keys {
		key := fmt.Sprintf("%v", k.Interface())
		if key == "" {
			continue
		}
		lenKeys++
		croots := append(roots, key)
		value := v.MapIndex(k)

		f := valueFromInterface(value.Interface())

		if validAndNotEmpty(f) && f.Type().Kind() == reflect.Struct {
			stringer, ok := value.Interface().(fmt.Stringer)
			if ok {
				structKey := strings.Join(sliceFormat(croots, e.Formatters), e.Separator)
				w[structKey] = stringer.String()
			}
			if !e.DisableTypePrefix {
				croots = append(croots, f.Type().Name())
			}
		}

		if err := e.fdumpInterface(w, value.Interface(), croots); err != nil {
			return err
		}
	}

	if e.ExtraFields.Len {
		nodeLen := append(roots, "__Len__")
		nodeLenFormatted := strings.Join(sliceFormat(nodeLen, e.Formatters), e.Separator)
		w[nodeLenFormatted] = lenKeys
	}
	if e.ExtraFields.DetailedMap {
		if len(roots) != 0 {
			structKey := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
			w[structKey] = i
		}
	}
	return nil
}

func (e *Encoder) fdumpStruct(w map[string]interface{}, s reflect.Value, roots []string) error {
	if e.ExtraFields.DetailedStruct {
		if e.ExtraFields.Len {
			nodeLen := append(roots, "__Len__")
			nodeLenFormatted := strings.Join(sliceFormat(nodeLen, e.Formatters), e.Separator)
			w[nodeLenFormatted] = s.NumField()
		}

		structKey := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
		if s.CanInterface() && len(roots) > 1 {
			w[structKey] = s.Interface()
		}
	}

	var atLeastOneField bool
	for i := 0; i < s.NumField(); i++ {
		k := reflect.ValueOf(i).Kind()
		if k == reflect.Ptr && reflect.ValueOf(i).IsNil() {
			if len(roots) == 0 {
				continue
			}
			k := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
			w[k] = ""
			atLeastOneField = true
			continue
		}

		if !s.Field(i).CanInterface() {
			continue
		}
		var croots []string
		var keyNameComputed bool
		if e.ExtraFields.UseJSONTag {
			tagValues := strings.Split(s.Type().Field(i).Tag.Get("json"), ",")
			if len(tagValues) > 0 && tagValues[0] != "omitempty" && tagValues[0] != "" {
				croots = append(roots, tagValues[0])
				keyNameComputed = true
			}
		}
		if !keyNameComputed {
			croots = append(roots, s.Type().Field(i).Name)
		}
		atLeastOneField = true
		if err := e.fdumpInterface(w, s.Field(i).Interface(), croots); err != nil {
			return err
		}
	}

	if !atLeastOneField {
		stringer, ok := s.Interface().(fmt.Stringer)
		if ok {
			structKey := strings.Join(sliceFormat(roots, e.Formatters), e.Separator)
			w[structKey] = stringer.String()
		}
	}

	return nil
}

// ToStringMap formats the argument as a map[string]string. It formats exactly the same as Dump.
func (e *Encoder) ToStringMap(i interface{}) (res map[string]string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			buf := make([]byte, 1<<16)
			runtime.Stack(buf, true)
		}
	}()
	ires := map[string]interface{}{}
	if err = e.fdumpInterface(ires, i, nil); err != nil {
		return
	}
	res = map[string]string{}
	for k, v := range ires {
		res[k] = printValue(v)
	}
	return
}

// ToMap dumps argument as a map[string]interface{}
func (e *Encoder) ToMap(i interface{}) (res map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			buf := make([]byte, 1<<16)
			runtime.Stack(buf, true)
		}
	}()
	res = map[string]interface{}{}
	if err = e.fdumpInterface(res, i, nil); err != nil {
		return
	}
	return
}

func (e *Encoder) ViperKey(s string) string {
	if e.Prefix != "" {
		s = strings.Replace(s, e.Prefix+e.Separator, "", 1)
	}
	s = strings.Replace(s, e.Separator, ".", -1)
	s = strings.ToLower(s)
	return s
}

func printValue(i interface{}) string {
	s, is := i.(string)
	if is {
		return s
	}
	ps, is := i.(*string)
	if is && ps != nil {
		return *ps
	}
	stringer, is := i.(fmt.Stringer)
	if is {
		return stringer.String()
	}
	btes, err := json.Marshal(i)
	if err == nil {
		compactedBuffer := new(bytes.Buffer)
		err := json.Compact(compactedBuffer, btes)
		if err != nil {
			return string(btes)
		}
		return compactedBuffer.String()
	}
	return fmt.Sprintf("%v", i)
}
