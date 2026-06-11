# WA contacts reverse notes

## Local stores

The Android app keeps two contact projections in `wa.db`:

- `wa_contacts`: WhatsApp-owned contact projection keyed by `jid`.
- `wa_address_book`: Android address-book projection enriched with WhatsApp status and JID mapping.

The app joins both projections with profile/group metadata tables when rendering or syncing contacts:

- `wa_vnames` for verified business names.
- `wa_group_descriptions` for group descriptions.
- `wa_group_admin_settings` for group flags.
- `wa_biz_profiles` for business profile metadata.

## Main app queries

Observed JADX references:

- `X/C0RT.java` and `X/C0RV.java`: read `wa_contacts` by JID, JID set, JID pattern, and phone number.
- `X/C37C.java`: reads `wa_address_book` for contact picker, sync upload batches, and native contacts.
- `X/C06530Sy.java`: persists address-book contacts and lists all DB contacts for sync.
- `X/C65822uV.java`: builds the address-book picker SQL.
- `X/AnonymousClass846.java`: builds `usync` IQ requests for phone/JID/LID/username contact lookups.

The minimal UI-safe field set for wa-app is therefore:

- `jid`
- `number`
- `display_name`
- `wa_name`
- `verified_name`
- `is_whatsapp_user`
- `is_reachable`
- derived `kind`

Raw Android contact IDs, table names, DB paths, and WA internal sync protocol flags stay internal to extractors and are not exposed through public proto.

## Contact profile pictures

Reverse target: `/Users/pood1e/workspace/wa-eng/app-release-re/app-release.apk`.

Observed app path:

- Individual contacts use `X/C08570aV.java` -> `X/C83N.smali`.
- The app sends an IQ:
  - `iq`: `xmlns="w:profile:picture"`, `type="get"`, `to="s.whatsapp.net"`, `target=<contact-jid>`, generated `id`.
  - `picture`: `type="image"` or `type="preview"`.
  - `picture@id` is included only when a positive cached photo id is known.
  - `picture@query="url"` is included for `image`; for `preview` it is only added for the app's FBID-migration targets (`X/C15K.A02`), not for ordinary LID contacts.
  - When available, profile-privacy enrichment adds `picture@common_gid` and a `picture/tctoken` child (`X/C21760xV.java`, `X/C83D.java`).
- Success parsing (`X/C83N.BuX`) reads `picture@id`, `picture@url`, `picture@direct_path`, `picture@hash`, and inline picture bytes. `404` and `410` mean no profile picture. Other IQ errors are treated as remote failures.
- The HTTP download path (`X/GXF.java`) downloads the returned `url` with the app User-Agent and bounded connect/read timeouts; `direct_path`/`hash` are stored as metadata.
- Group profile pictures are separate: `X/C9A4.java` sends `xmlns="w:g2"` to the group with `picture@query="url"`/`"blob"`.

Implementation note:

- wa-app mirrors the individual-contact request shape, tries the known photo id first, then retries without it so an API caller without local image cache can still fetch bytes after a `304`/unchanged-style response.
- PN JIDs are derived from normalized phone digits before falling back to the LID JID. This matches the app's contact model more closely than treating a display phone string as a raw JID.

## App-side filtering worth preserving

The app's WhatsApp-user contact query filters out special JIDs:

- `broadcast`
- `*@broadcast`
- `*@g.us` for user-only lists
- `*@temp`
- `*@newsletter`
- self JID

Address-book picker queries order by `display_name`, `jid`, and `phone_type`.

## LID to phone-number mapping

Observed reverse points from `workspace/wa-eng/app-release-re`:

- Main WA message proto `X/C30285DcY.java` uses `PROTOCOL_MESSAGE_FIELD_NUMBER = 12`.
- Protocol message `X/C30281DcU.java` uses `LID_MIGRATION_MAPPING_SYNC_MESSAGE_FIELD_NUMBER = 23`.
- `X/DYF.java` contains `encodedMappingPayload` at field `1`.
- `SendLidMigrationMappingSyncJob.java` shows this payload is gzip-compressed `X/C30110DZi`.
- `X/C30110DZi.java` has repeated `pnToLidMappings` at field `1` and `chatDbMigrationTimestamp` at field `2`.
- `X/C30165Dab.java` mapping entries use numeric `pn = 1`, `assigned_lid = 2`, and `latest_lid = 3`.
- History sync `X/C24912B0t.java` additionally contains `phoneNumberToLidMappings = 15` (`X/B03.java`: `pn_jid = 1`, `lid_jid = 2`) and `inlineContacts = 20` (`X/C24896B0d.java`: `pn_jid = 1`, `lid_jid = 2`, `full_name = 3`, `first_name = 4`, `username = 5`).
- Incoming chat nodes may carry `notify`, `display_name`, `sender_lid`, `sender_pn`, `participant_lid`, `participant_pn`, `peer_recipient_lid`, `peer_recipient_pn`, `recipient_latest_lid`, `username`, `peer_recipient_username`, and `participant_username` attrs; these are treated as opportunistic contact hints only.
- More LID aliases are surfaced outside the encrypted message body. The app stores username/name hints from:
  - broadcast recipients: `participants/to@peer_recipient_lid`, `peer_recipient_pn`, `peer_recipient_username` (`X/C164577Rd.java`);
  - status receipts: `participant`, `participant_pn`, `participant_username` (`X/C1Np.java`);
  - group notifications: `participant` + `participant_username`, child `author` + `author_username` (`X/C16240oO.java`).
  - incoming message stanza LID parser: `sender_lid`/`sender_pn`, `participant_lid`/`participant_pn`, `peer_recipient_lid`/`peer_recipient_pn`, `recipient_latest_lid`, `peer_recipient_username`, and `participant_username` (`X/C7SJ.java`);
  - peer and creator PN attrs used by protocol parsers: `peer_pn_jid` (`X/A3X.java`), `creator_pn` and `participant_pn` (`X/C214599hw.java`), `contact_pn` (`X/C22573A0j.java`), and inactive notification `sender_pn_jid` (`X/C23628Acm.java`, `X/C23627Acl.java`).
  wa-app now scans incoming chat-node attributes structurally, stores safe LID/PN/name/username hints at the native state level, and also keeps per-payload hints beside encrypted payload metadata. This matters because some useful alias updates arrive on non-message or plaintext notification nodes and would otherwise be lost before a later contact resolve.
- The local app also has durable mapping/display tables separate from the normal contact rows:
  - `jid_map` maps PN row IDs to LID row IDs and is read by `WaJidMapRepository` (`X/C06080Re.java`, `X/C0TE.java`).
  - `lid_display_name(lid_row_id, display_name, username)` carries display data for LID-only contacts (`X/C0LF.java`, `X/B3V.java`).
  - `contact_metadata(contact_lid, contact_pn, contact_username, contact_push_name)` is another non-message source for LID/PN/name hints (`X/C22573A0j.java`).
  wa-app does not expose these storage details in proto; it only projects the resolved account-local contact identity.
- App-state contact actions carry local contact display data and LID/PN pairs:
  - `contact_action` (`X/C24903B0k.java`): `full_name = 1`, `first_name = 2`, `lid_jid = 3`, `pn_jid = 5`, `username = 6`.
  - `pn_for_lid_chat_action` (`X/C2OM.java`): `pn_jid = 1`; the enclosing app-state key is required before it can be used as a mapping.
  - `lid_contact_action` (`X/B0O.java`): `full_name = 1`, `first_name = 2`, `username = 3`; this record has display data only and must not be treated as a LID/PN mapping by itself.
  - `out_contact_action` (`X/C44534Jpe.java`): `full_name = 1`, `first_name = 2`; the enclosing app-state key supplies the LID/PN side of the contact identity.
- App-state encrypted mutation values unwrap through `X/C44404JnF.java`:
  - `index = 1` stores the JSON key array built by `X/AbstractC61462n5.java`.
  - `value = 2` stores the `X/C2P0.java` app-state value.
  - Contact keys are shaped like `["contact", "<pn-jid>"]`; `X/C23591AcA.java` puts the paired LID in `contact_action.lid_jid`.
  - LID chat mapping keys are shaped like `["pnForLidChat", "<lid-jid>"]`; the PN is in `pn_for_lid_chat_action.pn_jid`.
  - LID display-name keys are shaped like `["lid_contact", "<lid-jid>"]`; display data is in `lid_contact_action`.
  - Out-contact keys use the same indexed app-state wrapper and carry display data through `out_contact_action`.

wa-app now extracts these as internal contact hints and projects them into the account-local `WAContact` record keyed by the LID JID. The public contract is unchanged; the UI should show the resolved phone-number/name when available and never surface the raw LID as the display title.

## 2026-06-09 dirty/app-state node pass

Reverse targets:

- `X/AbstractC34581eJ.java`: binary-node secondary token table contains the app-state and dirty-sync vocabulary used by incoming stanzas: `collection`, `sync`, `patch`, `patches`, `regular`, `regular_low`, `regular_high`, `critical_block`, `critical_unblock_low`, `w:sync:app:state`, `urn:xmpp:whatsapp:dirty`, `dirty`, `clean`, `return_snapshot`, and `keys`.
- `X/C192348eT.java`: the app builds a `clean` dirty-bit acknowledgement with `type="syncd_app_state"` after handling app-state dirtiness.
- `X/C41611pz.java`: `md-app-state` media uses the `"WhatsApp App State Keys"` HKDF info, confirming that app-state payloads are not ordinary chat message text.
- `X/C30074DXy.java`, `X/DZB.java`, `X/DYG.java`, and `X/C30141DaD.java`: protocol-message app-state key share carries repeated key records (`key_id`, `key_data`, fingerprint, timestamp). Full app-state patch decryption needs this key store; it must not be guessed from LID values or message text.
- `X/C24912B0t.java`: history sync payload carries `conversations` and `inline_contacts`; `inline_contacts` is the reverse class `C24896B0d` with `pn_jid = 1`, `lid_jid = 2`, `full_name = 3`, `first_name = 4`, `username = 5`.
- `X/C24914B0v.java`: history sync conversation records expose `id = 1`, `name = 13`, `display_name = 38`, `pn_jid = 39`, `lid_jid = 42`, and `username = 43`.
- `X/C24915B0w.java` + `X/C24916B0x.java`: history sync message records expose `push_name = 19`, `verified_biz_name = 37`, and a message key with `remote_jid = 1` / `participant = 4`; these can provide a safe LID-to-name hint even when the PN is still unavailable.
- `X/C210459b1.java` builds local contact metadata payloads with `X/C8PP.java`: `first_name = 1`, `last_name = 2`, `business_name = 3`, `sync_policy = 4`, `username = 6`, `phone_number = 7`, numeric `lid = 8`, and `version = 9`. The app usually wraps these rows in encrypted `X/C187268Ov.java` records (`iv = 1`, `key_id = 2`, `contact_metadata_encrypted = 3`), but plaintext/decoded appearances are safe contact hints when the sync-policy/version marker and a valid LID are present.

Implementation note: wa-app extends the binary-node token subset needed for dirty/app-state nodes and scans safe plaintext binary node payloads for the same contact-hint protobuf records already used after message decryption. Encrypted `enc` payloads and media/routing blobs are excluded from this plaintext scan. The contact hint recognizer now also covers history-sync conversation, inline-contact, and message-key/push-name records from the reverse classes above. This preserves the existing Signal/app-state boundaries while allowing future plaintext sync notifications to enrich contacts without logging or exposing raw JIDs, names, phone numbers, message bodies, keys, or tokens.

## 2026-06-08 active usync contact query

Reverse target: `AnonymousClass846.A0E(querySyncUsernameByLid)` and `C08180Zr.A05/A06/BuX`.

Findings:
- LID lookup uses a normal usync IQ, not a message body migration payload:
  - `iq`: `xmlns="usync"`, `type="get"`, generated `id`.
  - `usync`: `sid=sync_sid_query*`, `index="0"`, `last="true"`, `mode="query"`, `context="interactive"`.
  - `query`: request protocols include `username`, `contact`; `lid`/`business` are safe extra protocols for PN and verified-name enrichment.
  - `list/user`: request users carry `jid=<lid-jid>`.
  - App order is `query` first, then `list`; keep this order when probing because some servers return only echoed LID rows for non-canonical query shapes.
- Response parser in `C08180Zr.BuX` reads `usync/list/user` and `usync/side_list/user`:
  - `user@jid` is the primary jid; for LID requests this is expected to be `*@lid`.
  - `user@pn_jid`/`new_jid` provide PN mapping.
  - child `lid@val` may provide a LID when response is PN-centered.
  - child `username`, nested `username/username_info@username`, and `contact@username` provide username.
  - child `business@pn_jid` and `business/verified_name` can enrich PN and verified-name.
- Binary node token table must follow `AbstractC34581eJ.A00`; older partial token maps shift `mode/query/list/lid/usync/side_list/error` and make usync requests unreadable.

Implementation note:
- wa-app now has a bounded active resolver (`ResolveWAContacts`) that queries unresolved `*@lid` contacts through chatd usync and upserts only recovered contact projections. It does not log JIDs, names, phone numbers, message bodies, or reusable session material.

Follow-up:
- The first active query in the sandbox returned structurally valid `user` rows but no PN/name enrichment. To keep the reverse loop safe, the resolver now emits only protocol-shape logs: variant name, IQ type, user counts, attr/tag keys, JID domain classes, and PN/LID/name hint counts. It never prints raw JIDs, phone numbers, names, OTPs, message bodies, tokens, or session material.
- The resolver tries bounded protocol variants until the whole batch is enriched instead of treating echoed LID rows as resolved identities:
  - app-exact `interactive/query` with `username` + `contact`;
  - richer `interactive/query` with `username` + `contact` + `lid` + `business`;
  - `interactive/query` with `contact@addressing_mode=lid`, `lid`, `username`, and `business`;
  - LID-migration `message/query` and `notification/query` with `contact@addressing_mode=lid`;
  - `interactive` side-list probes with `sidelist@addressing_mode=lid` and with app-observed plain `sidelist`.
- `resolved_count` now counts contacts that actually gained a phone number, WA username, verified name, or non-fallback display name. Echoed `*@lid` rows without enrichment remain contacts but are not counted as resolved.
- Active usync is only an enrichment path. Network/proxy/dial/timeout failures must not make the contacts UI fail: wa-app returns the existing local projection and `queried_count` while leaving `resolved_count` unchanged. Non-retryable protocol-shape errors remain surfaced so reverse regressions are still visible.
- In the current sandbox, unresolved LID username usync responses are structurally valid but contain only echoed `user@jid` plus empty `username` children. The UI and API must therefore treat active usync as best-effort enrichment and must never surface a raw `LID ...` fallback as the contact title when no PN/name is returned.

## 2026-06-09 app-state mutation key and patch pass

Reverse targets:

- `X/C30285DcY.java`: main message `protocol_message = 12`.
- `X/C30281DcU.java`: protocol message `app_state_sync_key_share = 7`.
- `X/C30074DXy.java`: key-share wrapper has repeated `keys = 1`.
- `X/DZB.java`: key record uses `key_id = 1`, `key_data = 2`.
- `X/DYG.java`: key-id wrapper stores raw id bytes at field `1`.
- `X/C30141DaD.java`: key-data wrapper stores `key_data = 1`, `fingerprint = 2`, `timestamp = 3`.
- `sync.db` `crypto_info`: the app persists the same key material by `device_id`, `epoch`, `key_data`, `timestamp`, and `fingerprint`.
- `X/C46023Kak.java` and `KmpSyncdDecryptor.java`: app-state mutation keys are derived with HKDF info `WhatsApp Mutation Keys` into five 32-byte segments. The first three are used for index MAC, value AES-CBC key, and value HMAC key.
- `X/C44418JnT.java`: encrypted patch has `mutations = 2`, `key_id = 6`, and `client_debug_data = 9`.
- `X/C44387Jmy.java`, `X/C44397Jn8.java`, `X/C44379Jmq.java`, `X/C44380Jmr.java`, `X/C44378Jmp.java`: a mutation carries `operation = 1`, `record = 2`; the record carries index blob, value blob, and key id.
- `X/C44404JnF.java`: decrypted mutation value is the indexed sync-action wrapper (`index = 1`, `value = 2`, `padding = 3`, `version = 4`).
- `X/C44405JnG.java` and `X/C44399JnA.java`: snapshot/fatal-recovery payloads provide `collection_name = 2` and repeated mutation records whose `value = 1` is already the indexed sync-action wrapper.
- `X/DZK.java`: collection snapshots can be wrapped as `collection_snapshot = 1`, `is_compressed = 2`.

Implementation note: wa-app now persists app-state sync keys into the native profile state when a decrypted protocol message carries a key share. Contact hint extraction also handles plaintext app-state snapshots and decryptable encrypted mutations. Mutation decryption validates the data HMAC, AES-CBC/PKCS#7 padding, and index HMAC before parsing `C2P0`; undecryptable or unauthenticated mutations are ignored rather than guessed. The implementation records only derived contact projections and never logs or returns raw key material, fingerprints, message bodies, phone numbers, JIDs, or app-state payload bytes.
