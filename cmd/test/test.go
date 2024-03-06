package main

import (
	"fmt"
	"reflect"

	"github.com/emersion/go-imap"
	lua "github.com/yuin/gopher-lua"
)

type LuaMail struct {
	From        string
	To          string
	Subject     string
	Body        string
	Attachments []string
}

func main() {
	l := lua.NewState()
	defer l.Close()

	msg := imap.NewMessage(0, []imap.FetchItem{
		imap.FetchEnvelope,
	})
	msg.Envelope = &imap.Envelope{
		Subject: "test",
	}
	marshalToTable(l, msg)

	err := l.DoFile("test.lua")
	if err != nil {
		panic(err)
	}

	filterFunc := l.G.Global.RawGet(lua.LString("filter"))

	if filterFunc == lua.LNil {
		fmt.Println("filter not found")
		return
	}

	if filterFunc.Type() != lua.LTFunction {
		fmt.Println("filter is not a function")
		return
	}

	err = l.CallByParam(lua.P{
		Fn:      filterFunc,
		NRet:    1,
		Protect: true,
	}, createLuaMail(l))

	fmt.Println()

	if err != nil {
		panic(err)
	}

	ret := l.Get(-1)
	l.Pop(1)

	if s, ok := ret.(lua.LBool); ok {
		fmt.Println(s)
	} else {
		fmt.Println("not a bool")
	}
}

func createLuaMail(L *lua.LState) *lua.LTable {
	mt := L.NewTypeMetatable("luaMail")
	L.SetField(mt, "new", L.NewFunction(newLuaMail))
	return mt
}

func newLuaMail(L *lua.LState) int {
	mail := &LuaMail{}
	ud := L.NewUserData()
	ud.Value = mail
	L.SetMetatable(ud, L.GetTypeMetatable("luaMail"))
	L.Push(ud)
	return 1
}

func marshalToTable(L *lua.LState, mail interface{}) lua.LValue {
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
				fmt.Println(field.Name, v.Field(i).Interface())
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
			v.Field(i)
			arr.RawSetInt(i+1, marshalToTable(L, v))
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

// 	table := L.NewTable()
// 	for _, field := range  {
// 		L.SetField(table, field.Name, lua.LString(field.Name))
// 	}
// 	L.SetField(table, "from", lua.LString(mail.From))
// 	L.SetField(table, "to", lua.LString(mail.To))
// 	L.SetField(table, "subject", lua.LString(mail.Subject))
// 	L.SetField(table, "body", lua.LString(mail.Body))
// 	L.SetField(table, "attachments", lua.LString(mail.Attachments))
// 	return table
// }
