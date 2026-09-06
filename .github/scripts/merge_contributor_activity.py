import re
import subprocess
from pathlib import Path


def merge_chinese():
    path = Path("README.md")
    text = path.read_text(encoding="utf-8")
    activity = re.search(r"## 项目活跃度与贡献者\n.*?(?=\n---\n\n## 许可)", text, re.S)
    contributors = re.search(r"<!-- CONTRIBUTORS:START -->.*?<!-- CONTRIBUTORS:END -->", text, re.S)
    if not activity or not contributors:
        raise RuntimeError("README.md sections not found")
    activity_body = re.sub(r"^## 项目活跃度与贡献者\n\n", "", activity.group(0))
    text = text[:activity.start()] + text[activity.end():]
    contributors = re.search(r"<!-- CONTRIBUTORS:START -->.*?<!-- CONTRIBUTORS:END -->", text, re.S)
    contributor_block = contributors.group(0).replace("<!-- CONTRIBUTORS:START -->\n## 贡献者\n\n", "<!-- CONTRIBUTORS:START -->\n", 1)
    merged = "## 项目活跃度与贡献者\n\n" + contributor_block + "\n\n" + activity_body
    text = text[:contributors.start()] + merged + text[contributors.end():]
    path.write_text(text, encoding="utf-8")


def merge_static(filename, contributor_heading, next_heading, activity_heading, license_heading, contributor_label):
    path = Path(filename)
    text = path.read_text(encoding="utf-8")
    contributor = re.search(re.escape(contributor_heading) + r"\n.*?(?=\n" + re.escape(next_heading) + r")", text, re.S)
    activity = re.search(re.escape(activity_heading) + r"\n.*?(?=\n" + re.escape(license_heading) + r")", text, re.S)
    if not contributor or not activity:
        raise RuntimeError(f"{filename} sections not found")
    contributor_body = contributor.group(0)[len(contributor_heading):].strip()
    activity_body = activity.group(0)[len(activity_heading):].strip()
    merged = f"{activity_heading}\n\n### {contributor_label}\n\n{contributor_body}\n\n{activity_body}"
    first = contributor.start()
    text = text[:first] + merged + text[contributor.end():]
    activity = re.search(re.escape(activity_heading) + r"\n.*?(?=\n" + re.escape(license_heading) + r")", text, re.S)
    if activity and activity.start() != first:
        text = text[:activity.start()] + text[activity.end():]
    path.write_text(text, encoding="utf-8")


merge_chinese()
merge_static("README_EN.md", "## Contributors", "## Star History", "## Project Activity & Contributors", "---\n\n## License", "Contributors")
merge_static("README_RU.md", "## Участники", "## История звёзд", "## Активность проекта и участники", "---\n\n## Лицензия", "Участники")

base_ref = subprocess.check_output(["git", "rev-parse", "--abbrev-ref", "origin/HEAD"], text=True).strip() if False else None
base_branch = __import__("os").environ.get("GITHUB_BASE_REF", "main")
subprocess.run(["git", "fetch", "origin", base_branch, "--depth=1"], check=True)
unit_test = Path(".github/workflows/unit-test.yml")
unit_test.write_text(subprocess.check_output(["git", "show", f"origin/{base_branch}:.github/workflows/unit-test.yml"]), encoding="utf-8")
Path(__file__).unlink()
