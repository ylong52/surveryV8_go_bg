-- SV_3BHtJkfaEOEDj5c (id=2)
INSERT INTO site_page_fingerprints (site_id, page_id_pattern_regex, next_pattern, wait_after_click)
VALUES (
  2,
  '["/<legend>\\s*<label[^>]*class=\"[^\"]*QuestionText[^\"]*\"[^>]*>([\\s\\S]*?)<\\/label><\\/legend>/gi","/<legend>\\s*<div[^>]*class=\"[^\"]*QuestionText[^\"]*\"[^>]*>([\\s\\S]*?)<\\/div>\\s*<\\/legend>/gi"]',
  '/<input[^>]*id="NextButton"[^>]*>/',
  1500
);

-- sv_eM9M11Pt2XiyUNo (id=3)
INSERT INTO site_page_fingerprints (site_id, page_id_pattern_regex, next_pattern, wait_after_click)
VALUES (
  3,
  '["/<legend>\\s*<div[^>]*class=\"[^\"]*QuestionText[^\"]*\"[^>]*>([\\s\\S]*?)<\\/div>\\s*<\\/legend>/gi","/<legend><div[^>]*>([\\s\\S]*?)<\\/div><\\/legend>/gi","/<legend>\\s*<label[^>]*>([\\s\\S]*?)<\\/label><\\/legend>/gi"]',
  '/<input[^>]*id="NextButton"[^>]*>/',
  1500
);

-- SV_79T3rQ4xn7FNeiG (id=4)
INSERT INTO site_page_fingerprints (site_id, page_id_pattern_regex, next_pattern, wait_after_click)
VALUES (
  4,
  '["/<legend[^>]*>([\\s\\S]*?)<\\/legend>/gi","/<label[^>]*>([\\s\\S]*?)<\\/label>/gi","/<div[^>]*>([\\s\\S]*?)<\\/div>/gi"]',
  '/<input[^>]*id="btn_continue"[^>]*>/',
  1500
);
