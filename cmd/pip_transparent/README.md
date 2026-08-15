# PiP click-through toggle

This Windows command finds visible top-level windows whose title contains a
common Picture-in-Picture title and toggles the `WS_EX_TRANSPARENT` extended
window style. It also adds `WS_EX_LAYERED` when needed so Windows actually
passes hit tests through to the window underneath. Run it once to make the PiP
ignore mouse clicks and hover events; run it again to restore normal mouse
interaction.

```powershell
go build -o cmd\pip_transparent\pip_transparent.exe .\cmd\pip_transparent
.\cmd\pip_transparent\pip_transparent.exe
```

The program recognizes exact title variants such as `Picture in picture` and
`Picture-in-Picture`. Matching is deliberately exact so an ordinary browser
tab discussing Picture-in-Picture is not changed. Browsers that localize or
replace the PiP window title will need an additional title rule in
`isPiPTitle`.
