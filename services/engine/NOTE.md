Root cause:

- uv add f5-tts sebelumnya menambahkan f5-tts ke pyproject.toml, tapi karena konflik dependency dengan faster-whisper, setiap kali uv sync dijalankan (termasuk  
  auto-sync VS Code), langsung error dan block semua.

Yang sudah difix:

1. pyproject.toml — hapus f5-tts dari dependencies (diganti komentar)
2. VS Code tasks — ganti dari .venv/Scripts/activate; python main.py → .venv\Scripts\python.exe -X utf8 main.py (langsung pakai venv Python, tidak perlu activate,  
   tidak ada uv involvement)
3. f5-tts deps — reinstall via uv pip install --no-deps

Untuk ke depannya: jangan jalankan uv sync (atau uv add) karena akan hapus f5-tts lagi. Kalau perlu reinstall f5-tts: uv pip install f5-tts cached-path
x-transformers vocos torchdiffeq ema_pytorch unidecode pydub pypinyin rjieba hydra-core loguru wandb --no-deps
