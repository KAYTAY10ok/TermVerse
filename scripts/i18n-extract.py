#!/usr/bin/env python3
# Extracts translatable UI strings from the Go sources (used by TestFrenchCoverage).
# Usage: scripts/i18n-extract.py OUTPUT.json
import re,os,json,sys
import pathlib
root=str(pathlib.Path(__file__).resolve().parent.parent)
pats=[r'\bTf?\("((?:[^"\\]|\\.)*)"', r'(?:Desc|Label|Title|Name|Description|Question):\s*"((?:[^"\\]|\\.)*)"',
      r'func \(\w+ \*?\w+\) (?:name|desc)\(\) string\s*\{\s*return "((?:[^"\\]|\\.)*)"',
      r'NewInput\("((?:[^"\\]|\\.)*)"', r'(?:Open|Ask)\([^,]+,\s*"((?:[^"\\]|\\.)*)"', r'(?:Open|Ask)\("((?:[^"\\]|\\.)*)"']
found=set()
for d,_,fs in os.walk(root):
    if '.git' in d: continue
    for f in fs:
        if not f.endswith('.go') or f.endswith('_test.go') or f=='i18n_fr.go': continue
        src=open(os.path.join(d,f)).read()
        for p in pats:
            for m in re.finditer(p,src): found.add(m.group(1))
        # string slices for tabs / lists
        for m in re.finditer(r'(?:tabs|b64Modes|weekdays|months|tips|filters)\s*=\s*\[\]string\{([^}]*)\}',src,re.S):
            for x in re.findall(r'"((?:[^"\\]|\\.)*)"',m.group(1)): found.add(x)
        for m in re.finditer(r'\[\]string\{("[^}]*)\}',src):
            for x in re.findall(r'"((?:[^"\\]|\\.)*)"',m.group(1)):
                if re.search(r'[a-z]{3}',x): found.add(x)
found={x for x in found if re.search(r'[A-Za-z]{2}',x)}
json.dump(sorted(found),open(sys.argv[1],'w'),indent=0,ensure_ascii=False)
print(len(found), "strings")
