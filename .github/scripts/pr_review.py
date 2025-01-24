import os
import re
import json
import sys


def parse_function_signatures(pr_file):
    """
    Parse function definitions in a file to extract function names and return signatures.
    """
    functions = {}
    try:
        with open(pr_file, 'r') as file:
            content = file.readlines()
            for line in content:
                # Match function definitions with return types
                match = re.match(r'^\s*func\s+([a-zA-Z0-9_]+)\s*\(.*\)\s*\((.*)\)\s*{', line)
                if match:
                    func_name = match.group(1)
                    return_signature = match.group(2).split(",") if match.group(2) else []
                    functions[func_name] = [r.strip() for r in return_signature]
    except Exception as e:
        print(f"Error parsing file {pr_file}: {str(e)}")
    return functions


def check_function_calls(pr_file, helper_signatures):
    """
    Check if function calls in a file match the helper function return signatures.
    """
    notes = []
    try:
        with open(pr_file, 'r') as file:
            content = file.readlines()
            for i, line in enumerate(content):
                # Match function calls like `var, err := helper()`
                match = re.match(r'\s*(?:(?:[a-zA-Z0-9_]+, )*[a-zA-Z0-9_]+\s*):=\s*([a-zA-Z0-9_]+)\s*\(', line)
                if match:
                    func_name = match.group(1)
                    if func_name in helper_signatures:
                        # Get the expected return count
                        expected_count = len(helper_signatures[func_name])

                        # Count variables in the test
                        assigned_vars = line.split(":=")[0].split(",")
                        actual_count = len([v.strip() for v in assigned_vars if v.strip()])

                        if actual_count != expected_count:
                            notes.append({
                                "file": pr_file,
                                "line": i + 1,
                                "comment": f"Function '{func_name}' is expected to return {expected_count} value(s), but the test assigns {actual_count} variable(s)."
                            })
    except Exception as e:
        notes.append({
            "file": pr_file,
            "line": 0,
            "comment": f"Error reading file {pr_file}: {str(e)}"
        })
    return notes


def check_function_names(pr_file):
    """
    Check if function names follow Go standards, i.e., camelCase letters.
    """
    notes = []
    try:
        with open(pr_file, 'r') as file:
            content = file.readlines()
            for i, line in enumerate(content):
                # Match function definitions
                match = re.match(r'^\s*func\s+([a-zA-Z0-9_]+)\s*\(', line)
                if match:
                    func_name = match.group(1)

                    # Check if function is public or private
                    is_public = func_name[0].isupper()

                    # Check if function name is in CamelCase
                    if not re.match(r'^[A-Z]?[a-zA-Z0-9]+$', func_name):
                        notes.append({
                            "file": pr_file,
                            "line": i + 1,
                            "comment": f"{'Public' if is_public else 'Private'} function '{func_name}' does not follow CamelCase naming convention."
                        })
    except Exception as e:
        notes.append({
            "file": pr_file,
            "line": 0,
            "comment": f"Error reading file {pr_file}: {str(e)}"
        })
    return notes


def check_public_functions_missing_comments(pr_file):
    """
    Check if public functions have missing comments in Go files.
    """
    notes = []
    try:
        with open(pr_file, 'r') as file:
            content = file.readlines()
            for i, line in enumerate(content):
                # Match public functions
                public_func_match = re.match(r'^\s*func\s+([A-Z][a-zA-Z0-9_]*)\s*\(', line)
                if public_func_match:
                    func_name = public_func_match.group(1)
                    # Check if the line above is a comment
                    if i == 0 or not content[i - 1].strip().startswith("//"):
                        notes.append({
                            "file": pr_file,
                            "line": i + 1,
                            "comment": f"Public function '{func_name}' is missing a comment."
                        })
    except Exception as e:
        notes.append({
            "file": pr_file,
            "line": 0,
            "comment": f"Error reading file {pr_file}: {str(e)}"
        })
    return notes


def check_err_usage(pr_file):
    """
    Check for unused `err` variables in the code.
    """
    notes = []
    try:
        with open(pr_file, 'r') as file:
            content = file.readlines()
            for i, line in enumerate(content):
                line = line.strip()

                if re.match(r'\s*return\s+err\b', line):
                    continue

                if re.match(r'\s*(?:[a-zA-Z_]+, )?err\s*(?::=|=)\s*.*', line):
                    if i + 1 < len(content):
                        next_line = content[i + 1].strip()
                        if not re.search(r'(require\.NoError\(err\)|assert\.Error\(err\)|require\.Error\(err\))', next_line):
                            notes.append({
                                "file": pr_file,
                                "line": i + 2,
                                "comment": "Detected `err` assignment without proper error handling on the next line."
                            })
    except Exception as e:
        notes.append({
            "file": pr_file,
            "line": 0,
            "comment": f"Error reading file {pr_file}: {str(e)}"
        })
    return notes


if __name__ == "__main__":
    changed_files = sys.argv[1].split()

    review_notes = []
    helper_signatures = {}

    # Step 1: Parse helper functions
    for pr_file in changed_files:
        if pr_file.endswith(".go") and not pr_file.endswith("_test.go"):
            helper_signatures.update(parse_function_signatures(pr_file))

    # Step 2: Perform checks
    for pr_file in changed_files:
        if pr_file.endswith(".go"):
            # Check for public functions missing comments
            review_notes.extend(check_public_functions_missing_comments(pr_file))
            review_notes.extend(check_function_names(pr_file))
            review_notes.extend(check_err_usage(pr_file))
        
        if pr_file.endswith("_test.go"):
            review_notes.extend(check_function_calls(pr_file, helper_signatures))

    # Output results
    with open("review_notes.json", "w") as notes_file:
        json.dump(review_notes, notes_file, indent=2)
