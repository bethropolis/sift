package language

func javaFile(path, base string) Classification {
	return conventionByName(base, "Java source")
}

func kotlinFile(path, base string) Classification {
	return conventionByName(base, "Kotlin source")
}

func rubyFile(path, base string) Classification {
	return conventionByName(base, "Ruby source")
}

func phpFile(path, base string) Classification {
	return conventionByName(base, "PHP source")
}

func csharpFile(path, base string) Classification {
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
