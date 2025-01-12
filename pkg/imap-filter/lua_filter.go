package imap_filter

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"time"

	"github.com/sirupsen/logrus"
	lua "github.com/yuin/gopher-lua"
)

var filterFunctionName = "Filter"
var selectMailboxesFunctionName = "SelectMailboxes"

type LuaFilterConfig struct {
	ScriptsDir string `json:"scriptsDir" yaml:"scriptsDir"`
}

type LuaFilter struct {
	scriptsDir string
	luaFiles   []string
	luaStates  []*lua.LState
	lsFiles    lsFilesFunc
	readFile   readFileFunc
}

type lsFilesFunc func(string) ([]string, error)
type readFileFunc func(string) (string, error)

func NewLuaFilter(config LuaFilterConfig, lsFiles lsFilesFunc, readFile readFileFunc) *LuaFilter {
	return &LuaFilter{
		scriptsDir: config.ScriptsDir,
		lsFiles:    lsFiles,
		readFile:   readFile,
	}
}

func (f *LuaFilter) Close() {
	for _, l := range f.luaStates {
		l.Close()
	}
}

func (f *LuaFilter) Init() error {
	files, err := f.lsFiles(f.scriptsDir)
	if err != nil {
		log.WithError(err).Error("failed to list files")
		return nil
	}

	files = filterStrings(files, func(s string) bool {
		return path.Ext(s) == ".lua"
	})

	slices.Sort(files)
	f.luaFiles = files

	for _, luaFile := range f.luaFiles {
		luaFileContents, err := f.readFile(luaFile)
		if err != nil {
			log.WithError(err).Errorf("failed to read file %s", luaFile)
			continue
		}

		l, err := f.initLuaState(luaFileContents)
		if err != nil {
			log.WithError(err).Errorf("failed to init lua state for file %s", luaFile)
			continue
		}

		f.luaStates = append(f.luaStates, l)
	}

	return nil
}

func (f *LuaFilter) initLuaState(luaFileContents string) (l *lua.LState, err error) {
	l = lua.NewState()
	defer func() {
		if err != nil {
			l.Close()
		}
	}()

	err = l.DoString(luaFileContents)
	if err != nil {
		return
	}

	err = checkFuncExistance(l, filterFunctionName)
	if err != nil {
		return
	}

	return
}

func checkFuncExistance(l *lua.LState, name string) (err error) {
	fnc := l.G.Global.RawGet(lua.LString(name))

	if fnc == lua.LNil {
		err = fmt.Errorf(name + " not found")
		return
	}

	if fnc.Type() != lua.LTFunction {
		err = fmt.Errorf(name + " is not a function")
		return
	}

	return
}

func callLua(L *lua.LState, funcName string, args ...interface{}) (lua.LValue, error) {
	lArgs := make([]lua.LValue, len(args))
	for i, arg := range args {
		lArgs[i] = marchalToLValue(L, arg)
	}

	err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal(funcName),
		NRet:    1,
		Protect: true,
	}, lArgs...)

	if err != nil {
		return nil, err
	}

	ret := L.Get(-1)
	L.Pop(1)

	return ret, nil
}

// func newLuaMail(L *lua.LState) int {
// 	mail := &LuaMail{}
// 	ud := L.NewUserData()
// 	ud.Value = mail
// 	L.SetMetatable(ud, L.GetTypeMetatable("luaMail"))
// 	L.Push(ud)
// 	return 1
// }

func filterStrings(strings []string, filter func(string) bool) []string {
	var result []string
	for _, s := range strings {
		if filter(s) {
			result = append(result, s)
		}
	}
	return result
}

func (f *LuaFilter) SelectMailboxes() []string {
	resultMap := make(map[string]struct{})
	for _, l := range f.luaStates {
		err := checkFuncExistance(l, selectMailboxesFunctionName)
		if err != nil {
			continue
		}

		mailboxes, err := callLua(l, selectMailboxesFunctionName)
		if err != nil {
			log.WithError(err).Error("failed to call lua")
			continue
		}

		if mailboxes, ok := mailboxes.(*lua.LTable); ok {
			mailboxes.ForEach(func(k, v lua.LValue) {
				resultMap[v.String()] = struct{}{}
			})
		}
	}

	result := make([]string, 0, len(resultMap))
	for k := range resultMap {
		result = append(result, k)
	}

	return result
}

func (f *LuaFilter) Filter(mailbox string, message *Mail) (FilterResult, error) {
	for _, l := range f.luaStates {
		accept, err := callLua(l, filterFunctionName, message, mailbox)
		if err != nil {
			log.WithError(err).Error("failed to call lua")
			continue
		}

		if boolAccept, ok := accept.(lua.LBool); ok {
			if !bool(boolAccept) {
				return FilterResultReject, nil
			}
		} else if todo, ok := accept.(*lua.LTable); ok {
			if todo == nil {
				log.Error("tried to deal with nil LTable value from Filter()")
				continue
			}

			var kind = FilterResultKindNoop
			if s, ok := todo.RawGetString("kind").(lua.LString); ok {
				kind = FilterTypeResultFromString(string(s))
			}

			var target = ""
			if s, ok := todo.RawGetString("target").(lua.LString); ok {
				target = string(s)
			}

			return FilterResult{
				Kind:   kind,
				Target: target,
			}, nil
		}
	}

	return FilterResultAccept, nil
}

func marchalToLValue(L *lua.LState, mail interface{}) lua.LValue {
	return marchalToLValueDepth(L, mail, 0)
}

func marchalToLValueDepth(L *lua.LState, mail interface{}, depth int32) lua.LValue {
	if depth > 100 {
		logrus.Error("max depth reached")
		return lua.LNil
	}

	t := reflect.TypeOf(mail)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	v := reflect.ValueOf(mail)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if t == reflect.TypeOf(time.Time{}) {
		return lua.LString(mail.(time.Time).Format(time.RFC3339))
	}

	switch t.Kind() {
	case reflect.Struct:
		table := L.NewTable()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			vField := v.Field(i)

			if field.IsExported() {
				val := lua.LNil
				if vField.Kind() != reflect.Ptr && vField.Kind() != reflect.Map || !vField.IsNil() {
					val = marchalToLValueDepth(L, vField.Interface(), depth+1)
				}

				L.SetField(table, field.Name, val)
			}
		}
		return table
	case reflect.Map:
		if v.IsNil() {
			return lua.LNil
		}

		table := L.NewTable()
		for key, value := range v.MapKeys() {
			sKey := fmt.Sprintf("%v", key)
			L.SetField(table, sKey, marchalToLValueDepth(L, value, depth+1))
		}
		return table
	case reflect.String:
		return lua.LString(mail.(string))
	case reflect.Slice:
		arr := L.NewTable()
		for i := 0; i < v.Len(); i++ {

			arr.RawSetInt(i+1, marchalToLValueDepth(L, v.Index(i).Interface(), depth+1))
		}
		return arr
	case reflect.Uint8:
		return lua.LNumber(mail.(uint8))
	case reflect.Uint16:
		return lua.LNumber(mail.(uint16))
	case reflect.Uint32:
		return lua.LNumber(mail.(uint32))
	case reflect.Uint64:
		return lua.LNumber(mail.(uint64))

	case reflect.Int8:
		return lua.LNumber(mail.(int8))
	case reflect.Int16:
		return lua.LNumber(mail.(int16))
	case reflect.Int32:
		return lua.LNumber(mail.(int32))
	case reflect.Int64:
		return lua.LNumber(mail.(int64))

	default:
		logrus.WithField("type", t.Kind()).Error("unsupported type")
		return lua.LNil
	}
}
