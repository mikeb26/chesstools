#!/bin/bash

tail +2 a.tsv > all.tsv
tail +2 b.tsv >> all.tsv
tail +2 c.tsv >> all.tsv
tail +2 d.tsv >> all.tsv
tail +2 e.tsv >> all.tsv

cut -f3,2,1 -d$'\t' --output-delimiter=';' all.tsv | while read line
do
    IFS=';' read -ra fields <<< "$line"

    fen=$(echo "${fields[2]}" | ../pgn2fen)
    eco="${fields[0]}"
    openingName=${fields[1]}

    echo "$fen;$eco;$openingName"
done > all_fen.tsv

cat extra_fen.tsv>>all_fen.tsv
