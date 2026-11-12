# Inter 4.1

Source: https://github.com/rsms/inter/tree/v4.1/docs/font-files
License: SIL Open Font License 1.1; see OFL.txt.

Regular (400), Medium (500), SemiBold (600), Bold (700), Italic and BoldItalic
are the official static WOFF2 faces, decompressed to TTF without subsetting,
outline edits, renaming or weight synthesis. FontTools 4.68.0 / Brotli 1.2.1
were used once for format conversion; neither is a project dependency.

Go embeds and parses these assets directly with x/image. No OS font lookup,
network request, Python or browser engine is required at runtime. The font
supports Latin/Greek/Cyrillic; CJK fallback and complex shaping remain separate.
