# invitation-codecs

Invitation link and partner reference examples for `codec`.

The example uses Base62 for short URL-friendly invitation tokens, Base64URL for
callback state, and Hex for support-facing diagnostic references. These are
encodings, not encryption; do not put secrets in encoded values without a
separate security design.
