request:
- [x] add svg viewer (renders as image, data:image/svg+xml)
- [x] link in terminal: localhost/127.0.0.1 → in-app browser, other → system default browser
- [ ] paste image/file/folder from outside workspace → copies into workspace
- [ ] paste image/file to shell → inserts full path

bug:
- [x] opening folder not direct open folder, harus dua kali klik baru terbuka
- [x] modal rename is collapse explorer, should keep open sub folder opened
- [x] when editor is has block text, it cause issue overlap in shell, update: not only has block text, it happen for evey file open. the shell is collapse
- [not passed for shell in Session tab] when change tab from workspace to sessions, should open last session use for active session opened
- 
limitations:
- paste (explorer + shell + editor): cuma support 1 file/folder dari clipboard Finder. Multi-select clipboard (copy >1 item) fallback ke paste teks biasa. Solusi future: decode `public.file-url-list` plist atau Swift helper compile-once.
- paste button explorer: target selalu workspace root (folders[0]), belum bisa pilih folder target via selection.
