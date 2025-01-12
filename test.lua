-- type Mail struct {
--      From   []Address
--      Bcc    []Address
--      Cc     []Address
--      To     []Address
--      Sender []Address

--      Subject string
--      Date    time.Time
-- }

-- type Address struct {
--      Name  string
--      Email string
-- }



function filter(subject)
    print("filter called with: " .. subject.From[1])
    return true
end