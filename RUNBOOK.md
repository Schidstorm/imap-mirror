# Spam Filter Runbook

This file is a small working checklist for maintaining `filter.lua`.

## Goal

Refresh the local mail dump, inspect inbox spam over the last 3 months, update `filter.lua`, and clean up the generated dump afterwards.

## Todo

1. Dump the current mailbox state:

   ```sh
   go run ./cmd/dump --output-dir output
   ```

2. Inspect recent mails in `output/INBOX`.
   Use file modification time as receive time.
   Focus on the last 3 months.

3. Identify spam that still lands in `INBOX`.
   Prefer durable patterns over one-off literals:
   - sender domains or sender substrings for `rejectSenders`
   - sender regexes for `rejectSendersRegex`
   - subject patterns for `rejectSubjects`
   - focus on broad newsletter, job, paper/summary, and marketing spam, but keep the rules narrow enough to avoid blocking legitimate mail such as normal Facebook/Spotify sign-in messages

4. Update `filter.lua`.
   Keep changes minimal and targeted.
   Preserve legitimate invoices and other wanted mail.

5. Validate the Lua filter logic:

   ```sh
   go run ./cmd/test
   ```

6. Delete the generated dump when finished:

   ```sh
   rm -rf output
   ```

## Notes

- `rejectSubjects` uses Lua string patterns and is matched with `string.match`.
- Check mail headers first: `From`, `Sender`, `Return-Path`, `Reply-To`, and `Subject`.
- Avoid filtering only on cosmetic display names when the real sender domain is available.
- For Facebook-related mail, prefer matching explicit code-spam subjects such as `Facebook-Code` instead of blocking all Facebook traffic.