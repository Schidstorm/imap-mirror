package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type Mail struct {
	From   []string
	Bcc    []string
	Cc     []string
	To     []string
	Sender []string

	Subject string
	Date    time.Time
}

func main() {
	err := runFunctions("filter.lua", "Test")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runFunctions(luaFile, funcPattern string) error {
	l := lua.NewState()
	defer l.Close()

	err := l.DoFile(luaFile)
	if err != nil {
		return err
	}

	var functions []string
	l.ForEach(l.G.Global, func(key, value lua.LValue) {
		if value.Type() == lua.LTFunction {
			if strings.Contains(key.String(), funcPattern) {
				functions = append(functions, key.String())
			}
		}
	})

	var failedCount int
	for _, funcName := range functions {
		err := runFunction(l, funcName)
		if err != nil {
			fmt.Printf("[%s] error: %s\n", funcName, err)
			failedCount++
		} else {
			fmt.Printf("[%s] successfull\n", funcName)
		}
	}

	if failedCount == 0 {
		return nil
	}

	return fmt.Errorf("failed functions: %d", failedCount)
}

func runFunction(l *lua.LState, funcName string, args ...any) error {
	luaFunc := l.G.Global.RawGet(lua.LString(funcName))
	if luaFunc == lua.LNil {
		return fmt.Errorf("%s not found", funcName)
	}

	if luaFunc.Type() != lua.LTFunction {
		return fmt.Errorf("%s is not a function", funcName)
	}

	var luaArgs []lua.LValue
	for _, arg := range args {
		luaArgs = append(luaArgs, marshalToTable(l, arg))
	}

	err := l.CallByParam(lua.P{
		Fn:      luaFunc,
		NRet:    0,
		Protect: true,
	}, luaArgs...)
	if err != nil {
		return err
	}

	return nil
}

func marshalToTable(L *lua.LState, mail interface{}) lua.LValue {
	if v, ok := mail.(lua.LValue); ok {
		return v
	}

	t := reflect.TypeOf(mail)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	v := reflect.ValueOf(mail)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
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
					val = marshalToTable(L, vField.Interface())
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
			L.SetField(table, sKey, marshalToTable(L, value))
		}
		return table
	case reflect.String:
		return lua.LString(mail.(string))
	case reflect.Slice:
		arr := L.NewTable()
		for i := 0; i < v.Len(); i++ {
			arr.RawSetInt(i+1, marshalToTable(L, v.Index(i).Interface()))
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
		panic(fmt.Sprintf("unsupported type: %s", t.Kind()))

	}
}
