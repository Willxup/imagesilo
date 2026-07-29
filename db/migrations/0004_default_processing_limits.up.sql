UPDATE app_settings
SET max_total_pixels = 16000000,
    updated_at = unixepoch()
WHERE singleton = 1
  AND max_total_pixels = 40000000;
