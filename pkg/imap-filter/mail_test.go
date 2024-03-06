package imap_filter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testEml = `From: dominik@schidlowski.eu
To: DOMINIK@SCHIDLOWSKI.EU
Subject: =?utf-8?B?dGVzdFN1YmplY3Q=?=
Date: Sun, 18 Feb 2024 16:47:30 -0600
MIME-Version: 1.0
Content-Type: multipart/related;
	boundary="Mark=_-2079962875-103998143758"
X-Priority: 3
X-Envelope-From: <dominik@schidlowski.eu>
X-Envelope-To: <DOMINIK@SCHIDLOWSKI.EU>
X-Delivery-Time: 1708296451
X-UID: 47460
Return-Path: <dominik@schidlowski.eu>
ARC-Seal: i=2; a=rsa-sha256; t=1708296451; cv=pass;   d=strato.com; s=strato-dkim-0002;   b=cubkSaS0F+lfJUr+j5N7wZgOYBWSnsXy3Mlh4P8lRBeO5MhOxaVUKGWIawHXEOEnxj   YwwR3yQ03RWuDZlT1giQbpjv7maP4qzfA3IpHP/yY1p2Iax2Wo0DuLdslTPYjCahGRQY   2otCrYZ8QTQ58bmGn0qU3aFxxSlAIvfJtOTYmrIyBv12Ty8phTbVsh+fvaB9cxRMV7xx   QOE9hE2qEdzv5W6FTzvMQR2ct3A6LadWV/0SQli4Bnjj/6Z2tpTWpBzuEV08K+i7qj8u   DG2cFZa3W79J0+8vzreYbalrSdE6pNPTWdCsYLzB4bSGyb9aquaLsYwjhAZHFJ0JVvQS   eKeA==
ARC-Message-Signature: i=2; a=rsa-sha256; c=relaxed/relaxed; t=1708296451;   s=strato-dkim-0002; d=strato.com;   h=Message-ID:Date:Subject:To:From:Cc:Date:From:Subject:Sender;   bh=M0AUy4gB4GnwkZiobTV+NPNDpm6ClWOQzt6IBZVHuUE=;   b=evsbuWmVcfOuD8tMZlnaYnFIhzyCHKUh3CbZV0keoFU5lu8NQRb4X10q7JTv++wjjK   Amj2H8zDS/hLeHdIJJzA4+XszuK1Nc/yEyF1mYw56p9j4C0rufYPRtLzktBekFX5ewlA   yaUA219VQGxYmMSdgNHV+xuXV3OLhuy92fYig+7JY+yogtCdE3+fWBRxjY0fX12SipOM   5yo/IYEalbXdVxMsCBKAB+jq7BqUd6JhSihNWyiyn2fe2eqcXQwCRR8eGrh4iBdPCZVO   B0y+WY/iyz7BS5bkAHTbR30BFJDoI77UcSpb+oemG8oHX2xfPhW7pLSulm8Ftrvn5tjJ   8DFg==
ARC-Authentication-Results: i=2; strato.com;   dmarc=none header.from="schidlowski.eu";   arc=pass (i=1) smtp.remote-ip=81.169.146.160;   dkim=pass header.d="schidlowski.eu" header.s="strato-dkim-0003" header.a="ed25519-sha256";   dkim=pass header.d="schidlowski.eu" header.s="strato-dkim-0002" header.a="rsa-sha256";   dkim-adsp=pass;   spf=none smtp.mailfrom="dominik@schidlowski.eu"
Authentication-Results: strato.com;   dmarc=none header.from="schidlowski.eu";   arc=pass (i=1) smtp.remote-ip=81.169.146.160;   dkim=pass header.d="schidlowski.eu" header.s="strato-dkim-0003" header.a="ed25519-sha256";   dkim=pass header.d="schidlowski.eu" header.s="strato-dkim-0002" header.a="rsa-sha256";   dkim-adsp=pass;   spf=none smtp.mailfrom="dominik@schidlowski.eu"
X-RZG-Expurgate: clean/normal
X-RZG-Expurgate-ID: 149500::1708296451-529E19B0-F76141C8/0/0
X-RZG-CLASS-ID: mi00
Received-SPF: none   client-ip=81.169.146.160;   helo="mo4-p00-ob.smtp.rzone.de";   envelope-from="dominik@schidlowski.eu";   receiver=smtpin.rzone.de;   identity=mailfrom;
Received: from mo4-p00-ob.smtp.rzone.de ([81.169.146.160])   by smtpin.rzone.de (RZmta 49.11.2 OK)   with ESMTPS id T48b1901IMlVeAQ   (using TLSv1.3 with cipher TLS_AES_256_GCM_SHA384 (256 bits))   (Client CN "*.smtp.rzone.de", Issuer "Telekom Security ServerID OV Class 2 CA" (verified OK (+EmiG)))       (Client hostname verified OK)   for <DOMINIK@SCHIDLOWSKI.EU>;   Sun, 18 Feb 2024 23:47:31 +0100 (CET)
ARC-Seal: i=1; a=rsa-sha256; t=1708296451; cv=none;   d=strato.com; s=strato-dkim-0002;   b=XGoZNrFgYvI8Rs8QfDD6bU289MFv2GxG9ucl8hPQ3uOOmCJ9aQ0jGZ2plxg+zE992g   /nGpkWTA5nENWT74XWaorB4LyFOXZb2C7q5LnjfVRTgwBo/S+qIHJh8C9wiN6KEQJj2q   KC2jhIPvAm6KCJXbWW4zlGoZbHPA+jrgOhCMz1sVORzFSaqHSSIQNEj8Mzk0PHUYNN82   lQFCEki/29rzVP6XkPabK0DAuUpLnNDhPuy2U6JhRLNe+mu/rpIXo55HAPqu4CR++z13   egHp11rqt6lPMKzsfAp3ruoLZt9pz+rNE6j0x2RTvkQf3KeiVOBPRQPm6WechFOJhPBz   6Tkg==
ARC-Message-Signature: i=1; a=rsa-sha256; c=relaxed/relaxed; t=1708296451;   s=strato-dkim-0002; d=strato.com;   h=Message-ID:Date:Subject:To:From:Cc:Date:From:Subject:Sender;   bh=M0AUy4gB4GnwkZiobTV+NPNDpm6ClWOQzt6IBZVHuUE=;   b=UXDcPBesmplycvaWrAMl/OEiQO1N5tGVaESNyL+gdIzuKbboUbS/puEAunPx7zl32S   CV9w+EFwLDN1HzI/Y5jm9n0e7xO2qsPJDnskiAkSCI3gWKPGOnD+w1yJoYYjiI3i/oaq   4LmlZN+aBve6IWXz/6+JkJXlWky66Qdyn3YNNy3WOEZRbbaG4IX5aYqFWrm1Yp3ASOlI   IUsYX13DeXAVdEzg8OyaPaoy2uzBDlmGdtW6hfbsoIl3CyA20Hjr6giflRpAmHmYuRzE   stLxtkEKdUdyMojCGXi7BN4EppF9MLWs4moVo5qjMefSOUklvs1AMRT7ZHdwZsr7mQhw   A5vw==
ARC-Authentication-Results: i=1; strato.com;   arc=none;   dkim=none
X-RZG-CLASS-ID: mo00
DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; t=1708296451;   s=strato-dkim-0002; d=schidlowski.eu;   h=Message-ID:Date:Subject:To:From:Cc:Date:From:Subject:Sender;   bh=M0AUy4gB4GnwkZiobTV+NPNDpm6ClWOQzt6IBZVHuUE=;   b=epumpjEEqTOzeaLat1yBB5OsVyDItDsOaDKl9cGtxMdjWDhfbj6PMRx8WwAXu3aJ2x   6UeUcHSrZWov4ZfFlGAaz7qKkwYxr75sM+16CvG4jtxF6harWAAMnJCbL7d0pEGgSiTF   wDEIniYYa3+8aD3kPmX+LVqN5H6a4kQP0rVQaWJKZvQDdgF5tVa6XwGcj9MH9Q2yCT7H   Pd8bW16xdOM4H61ftg3yrcdmDl0GGniueJWjqNo5WSCGyyTjm67ewS5GUeUz/n7DQcuE   I8CqjAtTj8k5ow/78U1EYZrl8rA/JYbNya6Aknzvy2zKzbHbexBdi+a2gum0sTcSxrL6   r5Ew==
DKIM-Signature: v=1; a=ed25519-sha256; c=relaxed/relaxed; t=1708296451;   s=strato-dkim-0003; d=schidlowski.eu;   h=Message-ID:Date:Subject:To:From:Cc:Date:From:Subject:Sender;   bh=M0AUy4gB4GnwkZiobTV+NPNDpm6ClWOQzt6IBZVHuUE=;   b=GuvJ7qiJ7mL5kIwHt9NawqP3oXCEgl9Oq1LAifmRUJ0K/ykEKs0Td3GBcARjAI9raB   lKVIFqtsVp9Kn09WXwDQ==
X-RZG-AUTH: ":KGMJfE6hcN8N7v0LonlSqocfia1QBmPbPwpSydVKplTSu1osyGcE7W+22tm9WoO5qLqMLSA6S37EuTpxhQ=="
Received: from AM8PR09MB5445.eurprd09.prod.outlook.com   by smtp.strato.de (RZmta 49.11.2 AUTH)   with ESMTPSA id z7dfeb01IMlVXqA(using TLSv1.3 with cipher TLS_AES_256_GCM_SHA384 (256 bits))(Client did not present a certificate)   for <DOMINIK@SCHIDLOWSKI.EU>;   Sun, 18 Feb 2024 23:47:31 +0100 (CET)
Thread-Topic: testSubject
Thread-Index: AQHaYrxyTw+bSzx4i0ikvrB/JwwWQw==
X-MS-Exchange-MessageSentRepresentingType: 1
Accept-Language: en-US, de-DE
Content-Language: en-US
X-MS-Has-Attach: 
X-MS-Exchange-Organization-SCL: -1
X-MS-TNEF-Correlator: 
X-MS-Exchange-Organization-RecordReviewCfmType: 0
msip_labels: 

This is a multi-part message in MIME format.

--Mark=_-2079962875-103998143758
Content-Type: text/plain;
	charset="utf-8"
Content-Transfer-Encoding: base64

dGVzdG1haWw=

--Mark=_-2079962875-103998143758
Content-Type: text/html;
	charset="utf-8"
Content-Transfer-Encoding: base64

PFNUWUxFPg0KcHJlIHsNCndoaXRlLXNwYWNlOiBwcmUtd3JhcDsgLyogY3NzLTMgKi8NCndoaXRl
LXNwYWNlOiAtbW96LXByZS13cmFwICFpbXBvcnRhbnQ7IC8qIE1vemlsbGEsIHNpbmNlIDE5OTkg
Ki8NCndoaXRlLXNwYWNlOiAtcHJlLXdyYXA7IC8qIE9wZXJhIDQtNiAqLw0Kd2hpdGUtc3BhY2U6
IC1vLXByZS13cmFwOyAvKiBPcGVyYSA3ICovDQp3b3JkLXdyYXA6IGJyZWFrLXdvcmQ7IC8qIElu
dGVybmV0IEV4cGxvcmVyIDUuNSsgKi8NCn0NCjwvU1RZTEU+DQo8aHRtbD4NCjxoZWFkPg0KDQo8
c3R5bGUgdHlwZT0idGV4dC9jc3MiIHN0eWxlPSJkaXNwbGF5Om5vbmU7Ij4gUCB7bWFyZ2luLXRv
cDowO21hcmdpbi1ib3R0b206MDt9IDwvc3R5bGU+DQo8L2hlYWQ+DQo8Ym9keSBkaXI9Imx0ciI+
DQo8ZGl2IGNsYXNzPSJlbGVtZW50VG9Qcm9vZiIgc3R5bGU9ImZvbnQtZmFtaWx5OiBBcHRvcywg
QXB0b3NfRW1iZWRkZWRGb250LCBBcHRvc19NU0ZvbnRTZXJ2aWNlLCBDYWxpYnJpLCBIZWx2ZXRp
Y2EsIHNhbnMtc2VyaWY7IGZvbnQtc2l6ZTogMTJwdDsgY29sb3I6IHJnYigwLCAwLCAwKTsiPg0K
dGVzdG1haWw8L2Rpdj4NCjwvYm9keT4NCjwvaHRtbD4=

--Mark=_-2079962875-103998143758--

`

func TestParse(t *testing.T) {

	m, err := fromEmlFileBytes([]byte(testEml))

	assert.NoError(t, err)
	// assert.Equal(t, 1, len(m.From))
	// assert.Equal(t, "dominik@schidlowski.eu", m.From[0].Email)
	assert.Equal(t, 1, len(m.To))
	assert.Equal(t, "DOMINIK@SCHIDLOWSKI.EU", m.To[0].Email)
	assert.Equal(t, "testSubject", m.Subject)
	assert.Equal(t, time.Time(time.Date(2024, time.February, 18, 22, 47, 30, 0, time.UTC)), m.Date.UTC())

}
