package language

import "strings"

func javaFile(path, base string) Classification {
	return jvmFile(path, base, "Java source", ".java")
}

func kotlinFile(path, base string) Classification {
	return jvmFile(path, base, "Kotlin source", ".kt")
}

func rubyFile(path, base string) Classification {
	return conventionByName(base, "Ruby source")
}

func phpFile(path, base string) Classification {
	return conventionByName(base, "PHP source")
}

func csharpFile(path, base string) Classification {
	if strings.HasSuffix(base, "test.cs") || strings.HasSuffix(base, "tests.cs") {
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: "C# test file"}
	}
	if base == "program.cs" || base == "startup.cs" {
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: "C# application entrypoint"}
	}
	return conventionByName(base, "C# source")
}

func cFile(path, base string) Classification {
	return conventionByName(base, "C source")
}

func cppFile(path, base string) Classification {
	return conventionByName(base, "C++ source")
}

func swiftFile(path, base string) Classification {
	return conventionByName(base, "Swift source")
}

func conventionByName(base, label string) Classification {
	if base == "main.java" || base == "main.kt" || base == "main.rb" || base == "main.php" || base == "main.cs" || base == "main.c" || base == "main.cpp" || base == "main.swift" {
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.80, Reason: label + " entrypoint"}
	}
	return Classification{Role: RoleImpl, Confidence: 0.45, Reason: label}
}

func jvmFile(path, base, label, ext string) Classification {
	if strings.Contains(path, "/src/test/") || strings.HasSuffix(base, "test"+ext) || strings.HasSuffix(base, "tests"+ext) || strings.HasSuffix(base, "it"+ext) {
		return Classification{Role: RoleTest, Adjustment: -0.12, Confidence: 0.90, Reason: label + " test file"}
	}
	if base == "application"+ext || base == "main"+ext {
		return Classification{Role: RoleEntrypoint, Adjustment: 0.16, Confidence: 0.85, Reason: label + " entrypoint"}
	}
	if strings.HasSuffix(base, "controller"+ext) || strings.HasSuffix(base, "service"+ext) || strings.HasSuffix(base, "api"+ext) {
		return Classification{Role: RoleAPI, Adjustment: 0.10, Confidence: 0.75, Reason: label + " API layer"}
	}
	return conventionByName(base, label)
}
