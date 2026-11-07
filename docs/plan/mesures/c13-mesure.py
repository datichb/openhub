#!/usr/bin/env python3
"""C13 measure: comment lines of .go files that carry a plan code.

Usage (from the cli/ directory of a worktree):
  python3 c13-mesure.py [-v] [--exclude PREFIX ...] PATH [PATH ...]
Examples:
  C13a scope (everything except the C13b packages):
    python3 c13-mesure.py -v --exclude cmd internal/tui internal/archtest internal/i18n \
      internal/teamstate internal/tracker internal/mcp internal/mcpresolve internal/notify internal/services -- .
  C13b scope:
    python3 c13-mesure.py -v cmd internal/tui internal/archtest internal/i18n internal/teamstate \
      internal/tracker internal/mcp internal/mcpresolve internal/notify internal/services

A "plan code" line is a Go comment (// or trailing //, found by a small lexer so
that URLs in strings are ignored) that contains one of: A12, B3b, N17, R6, P3-T30,
C0.5, Q3-2, E14-M2, DC24, QB8, BL-22, LD-J, p6-debts / r5-fixes style lot names,
v5-fix-*, "lot X", "vague N", "wave N", "v5 phase/critical/corrections/finalisation/
hardening/finitions", v5.0 / v5.0.1 / v5.1 version mentions. NOT counted (kept on purpose):
audit ids S-xx / A-xx / F-xx / U-xx (docs/dev/audit-v5.md), CP-Rn, CA-n, ADR-nnn, and
technical tokens (SHA256, AES256, X25519, UTF8, H1…). In evalrun, S1..S77 are eval case ids;
in beads/specdoc, P0..P3 / T1 are priorities / tasks of the example.
Known false positives to ignore: comments quoting "v5.0.0" as a version example
(internal/semver), "C1 codes" (termtext), "V4" (credproxy/sigv4.go).
"""
import re,sys,os
KEEP=re.compile(r'\b(?:[A-Z]-\d{2}|N-\d|CP-R\d+|CA-\d|ADR-\d+)\b')
TECH=re.compile(r'SHA-?\d+|AES-?\d+|X25519|\bS3\b|P-256|UTF-?\d+|HTTP/\d|IPv[46]|OAuth2?|ISO\d+|RFC\d+|E2E|\bH[1-6]\b|MD5|ECDSA|ED25519|P256|\bV[124]\b|SigV4|U\+[0-9A-F]+')
PC=re.compile(r'\b(?:[pqr]\d+-[a-z]+(?:-[a-z]+)*|LD-J|[A-Z]{1,3}\d{1,2}[a-z]?(?:\.\d)?(?:-[A-Z]?\d+[a-z]?)?|v5-fix-\w+|[Ll]ots? [A-Z]\d*\w*|[Vv]agues? \d+|[Ww]aves? \d+|v5 (?:phase|critical|corrections|finalisation|hardening|finitions)\b|v5\.\d(?:\.\d)?|BL-\d+|\bDC\d+|\bP3-T\d+)\b')
def comments(lines):
    raw=False;block=False
    for l in lines:
        i=0;n=len(l);cs=None
        while i<n:
            ch=l[i]
            if block:
                j=l.find('*/',i)
                if j<0: break
                block=False;i=j+2;continue
            if raw:
                j=l.find('`',i)
                if j<0: break
                raw=False;i=j+1;continue
            if ch=='`': raw=True;i+=1;continue
            if ch=='"':
                i+=1
                while i<n and l[i]!='"': i+=2 if l[i]=='\\' else 1
                i+=1;continue
            if ch=="'":
                i+=1
                while i<n and l[i]!="'": i+=2 if l[i]=='\\' else 1
                i+=1;continue
            if l.startswith('//',i): cs=i;break
            if l.startswith('/*',i): block=True;i+=2;continue
            i+=1
        yield l,cs
def hits(path):
    out=[]
    L=open(path,encoding='utf-8').read().split('\n')
    for n,(l,cs) in enumerate(comments(L),1):
        if cs is None: continue
        c=TECH.sub(' ',KEEP.sub(' ',l[cs:]))
        m=PC.findall(c)
        if 'evalrun/' in path: m=[x for x in m if not re.fullmatch(r'S\d+',x)]
        if 'beads/beads.go' in path or 'specdoc/' in path: m=[x for x in m if not re.fullmatch(r'[PT]\d',x)]
        if m: out.append((n,l.strip()))
    return out
def main(argv):
    verbose='-v' in argv; argv=[a for a in argv if a!='-v']
    excl=[];paths=[]
    if '--exclude' in argv:
        i=argv.index('--exclude'); rest=argv[i+1:]
        if '--' in rest:
            j=rest.index('--'); excl=rest[:j]; paths=argv[:i]+rest[j+1:]
        else: excl=rest; paths=argv[:i]
    else: paths=argv
    files=[]
    for p in paths:
        if os.path.isfile(p): files.append(p)
        else:
            for r,_,fs in os.walk(p):
                for f in fs:
                    if f.endswith('.go'): files.append(os.path.join(r,f))
    files=sorted(f[2:] if f.startswith('./') else f for f in files)
    files=[f for f in files if not any(f==e or f.startswith(e.rstrip('/')+'/') for e in excl) and '/hubcontent/hub/' not in f]
    tot=0;nf=0
    for f in files:
        h=hits(f)
        if h:
            nf+=1;tot+=len(h)
            if verbose:
                for n,l in h: print(f'{f}:{n}:{l}')
    print(f'comment lines with a plan code: {tot} in {nf} files ({len(files)} .go files scanned)')
main(sys.argv[1:])
