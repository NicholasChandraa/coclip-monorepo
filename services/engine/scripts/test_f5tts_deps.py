"""
Test script to identify missing f5-tts runtime dependencies.
Run: .venv\Scripts\python.exe -X utf8 scripts\test_f5tts_deps.py
"""

# f5-tts runtime deps (excludes training/eval extras we don't need)
DEPS = [
    ("loguru", "loguru"),
    ("ema_pytorch", "ema-pytorch"),
    ("vocos", "vocos"),
    ("torchdiffeq", "torchdiffeq"),
    ("pydub", "pydub"),
    ("unidecode", "unidecode"),
    ("tomli", "tomli"),
    ("pypinyin", "pypinyin"),
    ("rjieba", "rjieba"),
    ("x_transformers", "x-transformers"),
    ("transformers_stream_generator", "transformers_stream_generator"),
]

missing = []
ok = []

for module, pkg in DEPS:
    try:
        __import__(module)
        ok.append(pkg)
    except ImportError:
        missing.append(pkg)

print(f"\n✅ OK ({len(ok)}): {', '.join(ok)}")
print(f"\n❌ Missing ({len(missing)}): {', '.join(missing) if missing else 'none'}")

if missing:
    print(f"\nInstall with:\n  uv pip install {' '.join(missing)} --no-deps")

# Now try the actual f5-tts import chain
print("\n--- Testing f5_tts import ---")
try:
    from f5_tts.model import DiT
    print("✅ f5_tts.model.DiT OK")
except Exception as e:
    print(f"❌ f5_tts.model.DiT FAILED: {e}")

try:
    from f5_tts.infer.utils_infer import load_model
    print("✅ f5_tts.infer.utils_infer OK")
except Exception as e:
    print(f"❌ f5_tts.infer.utils_infer FAILED: {e}")
