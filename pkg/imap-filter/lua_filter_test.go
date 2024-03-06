package imap_filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	filter := NewLuaFilter(LuaFilterConfig{ScriptsDir: "scripts"}, func(d string) ([]string, error) {
		assert.Equal(t, d, "scripts")
		return []string{}, nil
	}, func(string) (string, error) {
		return "", nil
	},
	)

	filter.Init()
	filter.Close()
}

func TestFilterParse(t *testing.T) {
	filter := NewLuaFilter(LuaFilterConfig{ScriptsDir: "scripts"}, func(d string) ([]string, error) {
		return []string{
			"scripts/test.lua",
		}, nil
	}, func(string) (string, error) {
		return `
		function Filter(mail, mailbox)
			return true
		end
		`, nil
	},
	)

	assert.Nil(t, filter.Init())
	assert.Equal(t, 1, len(filter.luaStates))

	filter.Close()
}

func TestFilterParseError(t *testing.T) {
	filter := NewLuaFilter(LuaFilterConfig{ScriptsDir: "scripts"}, func(d string) ([]string, error) {
		return []string{
			"scripts/test.lua",
		}, nil
	}, func(string) (string, error) {
		return `
		function filteasdasdasdr(mail, mailbox)
			return true
		end
		`, nil
	},
	)

	assert.Nil(t, filter.Init())
	assert.Equal(t, 0, len(filter.luaStates))

	filter.Close()
}

func TestFilterFilter(t *testing.T) {

	filter := NewLuaFilter(LuaFilterConfig{ScriptsDir: "scripts"}, func(d string) ([]string, error) {
		return []string{
			"scripts/test.lua",
		}, nil
	}, func(string) (string, error) {
		return `
		function Filter(mail, mailbox)
			return mail.Subject ~= "test"
		end
		`, nil
	},
	)

	assert.NoError(t, filter.Init())
	exampleMail := buildMail().Subject("test").From(Address{
		Name:  "dominik",
		Email: "dominik@schidlowski.eu",
	}).Build()

	res, err := filter.Filter("", exampleMail)
	assert.Nil(t, err)
	assert.Equal(t, FilterResultReject, res)

	filter.Close()
}

func TestFilterFilterReal(t *testing.T) {

	filter := NewLuaFilter(LuaFilterConfig{ScriptsDir: "scripts"}, func(d string) ([]string, error) {
		return []string{
			"scripts/test.lua",
		}, nil
	}, func(string) (string, error) {
		return `
		blacklist = {
			"dominik"
		}
			

		function Filter(mail, mailbox)
			for i, blacklistAddr in ipairs(blacklist) do
				print (blacklistAddr)

				for j, from in ipairs(mail.From) do
					print (from.Address)
					if string.find(from.Email, blacklistAddr) then
						return false
					end
				end
				
			end

			return true
		end
		`, nil
	},
	)

	assert.NoError(t, filter.Init())

	exampleMail := buildMail().Subject("test").From(Address{
		Name:  "dominik",
		Email: "dominik@schidlowski.eu",
	}).Build()
	res, err := filter.Filter("", exampleMail)
	assert.Nil(t, err)
	assert.Equal(t, FilterResultReject, res)

	filter.Close()
}
