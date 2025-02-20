-- type Mail struct {
-- 	From   []Address
-- 	Bcc    []Address
-- 	Cc     []Address
-- 	To     []Address
-- 	Sender []Address

-- 	Subject string
-- 	Date    time.Time
-- }

-- type Address struct {
-- 	Name  string
-- 	Email string
-- }

local onlyMailboxes = {
    "INBOX",
}

local rechnungenMailbox = "INBOX.Rechnungen"
local rechnungenSenders = {
    "service@paypal.de"
}

local rechnungenSubjects = {
    "Rechnung",
    "Invoice"
}

local rejectSenders = {
    "newsletter",
    "service.web-explorer.de",
    "produktnews@fleurop.de",
    "devevents@cloud.ebnermedia.de",
    "membership@sunspec.org",
    "service-news@1und1.de",
    "smartereveryday.com",
    "jenifer@aicapital.ae",
    "jobs@mail.xing.com",
    "dev@kubernetes.io",
    "kwk@1und1.de",
    "atmnews",
    "aws-marketing-email-replies@amazon.com",
    "news-noreply@google.com",
    "connect.etoro.com",
    "jem@fgkplaw.com",
    "no-reply@2r6ci.m-sender-sib.com",
    "junico.de",
    "info@ionos.de",
    "info@reply.de.shop-canda.com",
    "codeanywhere.net",
    "komoot.de",
    "news@nvidia.com",
    "sunspec.org",
    "loremachine@substack.com",
    "no-reply@shelly.cloud",
    "kwk@1und1.de",
    "gaming@nvgaming.nvidia.com",
    "product-line.de",
    "info@ionos.de",
    "informer@daily.dev",
    "traffic-meile.de"
}

local rejectSendersRegex = {
    ".*news.*@.*",
}

local rejectSubjects = {
    ".*news.*@.*",
    ".*Sichern +Sie +sich.*",
    "Diat",
}

local function assertEqual(a, b)
    if a ~= b then
        error("Expected " .. a .. " to be equal to " .. b)
    end
end

local function stringContains(str, substr)
    return string.find(str, substr, nil, true) ~= nil
end

function TestStringContains()
    assertEqual(stringContains("hello world", "world"), true)
    assertEqual(stringContains("hello world", "worlds"), false)
    assertEqual(stringContains("hello world", "hello"), true)
    assertEqual(stringContains("hello world", "hell"), true)
    assertEqual(stringContains("hello world", ""), true)
    assertEqual(stringContains("", "world"), false)
end

local function addressesContainsEmail(addrs, substr)
    for _, addr in ipairs(addrs) do
        if stringContains(addr.Email, substr) then
            return true
        end
    end
    return false
end

function TestAddressesContainsEmail()
    local addrs = {
        { Email="test@google.com" },
        { Email="keinTest@heise.de" },
    }

    assertEqual(addressesContainsEmail(addrs, "google"), true)
    assertEqual(addressesContainsEmail(addrs, "heise"), true)
    assertEqual(addressesContainsEmail(addrs, "test"), true)
    assertEqual(addressesContainsEmail(addrs, "keinTest"), true)
    assertEqual(addressesContainsEmail(addrs, "heise.de"), true)
    assertEqual(addressesContainsEmail(addrs, "google.com"), true)
    assertEqual(addressesContainsEmail(addrs, "heise.com"), false)
end

-- https://www.lua.org/manual/5.1/manual.html#5.4.1
local function addressesContainsEmailRegex(addrs, substr)
    for _, addr in ipairs(addrs) do
        if string.match(addr.Email, substr) then
            return true
        end
    end
    return false
end

function TestAddressesContainsEmailRegex()
    local addrs = {
        { Email="test@google.com" },
        { Email="keinTest@heise.de" },
    }

    assertEqual(addressesContainsEmailRegex(addrs, "goog.*"), true)
    assertEqual(addressesContainsEmailRegex(addrs, ".*heise.*"), true)
    assertEqual(addressesContainsEmailRegex(addrs, ".*[tT]est.*"), true)
end


local function doMailboxesContain(mailboxes, mailbox)
    for _, mb in ipairs(mailboxes) do
        if mb == mailbox then
            return true
        end
    end
    return false
end

function TestDoMailboxesContain()
    assertEqual(doMailboxesContain({"INBOX"}, "INBOX"), true)
    assertEqual(doMailboxesContain({"INBOX"}, "INBOX.Rechnungen"), false)
    assertEqual(doMailboxesContain({"INBOX", "INBOX.Rechnungen"}, "INBOX.Rechnungen"), true)
    assertEqual(doMailboxesContain({"INBOX", "INBOX.Rechnungen"}, "INBOX"), true)
    assertEqual(doMailboxesContain({"INBOX", "INBOX.Rechnungen"}, "INBOX.Something"), false)
end

local function accept()
    return { kind="noop" }
end

local function reject()
    return { kind="delete" }
end

local function move(target)
    return { kind="move", target=target }
end

local function containsFrom(subject, item)
    if addressesContainsEmail(subject.Sender, item) then
        return true
    end

    if addressesContainsEmail(subject.From, item) then
        return true
    end

    return false
end

local function containsFromRegex(subject, item)
    if addressesContainsEmailRegex(subject.Sender, item) then
        return true
    end

    if addressesContainsEmailRegex(subject.From, item) then
        return true
    end

    return false
end

local function doesMatch(subject, list, matcher) 
    for _, item in ipairs(list) do
        if matcher(subject, item) then
            return true
        end
    end

    return false
end

function Filter(subject, mailbox)
    if mailbox ~= "INBOX" then
        return accept()
    end

    if not doMailboxesContain(onlyMailboxes, mailbox) then
        return accept()
    end

    -- MOVE rechnungen
    if doesMatch(subject, rechnungenSenders, containsFrom) then
        print("FOUND IT! Moving mail")
        return move(rechnungenMailbox)
    end

    -- REJECT by sender
    if doesMatch(subject, rejectSenders, containsFrom) then
        print("FOUND IT! Rejecting mail (sender)")
        return reject()
    end

    -- REJECT by sender regex
    if doesMatch(subject, rejectSendersRegex, containsFromRegex) then
        print("FOUND IT! Rejecting mail (regex)")
        return reject()
    end

    -- REJECT by subject
    for _, sub in ipairs(rejectSubjects) do
        if stringContains(subject.Subject, sub) then
            print("FOUND IT! Rejecting mail (subject)")
            return reject()
        end
    end 

    return accept()
end

function TestFilter()
    local subject = {
        From = { { Email="newsletter@google.com" } },
        Sender = { },
        Subject = "Hello world",
    }

    assertEqual(Filter(subject, "INBOX").kind, "delete")
    assertEqual(Filter(subject, "INBOX.Rechnungen").kind, "noop")

end

function SelectMailboxes()
    return onlyMailboxes
end




