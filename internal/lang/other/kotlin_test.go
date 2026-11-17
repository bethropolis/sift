//go:build cgo

package langother

import (
	"strings"
	"testing"

	"github.com/bethropolis/sift/internal/lang/signature"
	"github.com/bethropolis/sift/internal/lang/types"
)

func TestKotlinSignaturesCompression(t *testing.T) {
	code := `package com.example.ui

import com.example.models.User as AppUser

@Entity(tableName = "users")
class DetailViewModel {
    fun load() {
        println("heavy implementation")
    }

    fun isReady(): Boolean = true

    fun checkStatus() = "OK"

    val formattedTitle: String
        get() {
            return "title".uppercase()
        }

    var count: Int = 0
        get() = 0
        set(value) {
            field = value
        }

    var readOnlyOutside: String = ""
        private set

    companion object {
        fun create(): DetailViewModel {
            return DetailViewModel()
        }
    }
}

object NetworkClient {
    fun fetch() {
        println("fetching")
    }
}

@Composable
@Preview
fun DetailScreen(user: AppUser) {
    Text("User detail")
}
`
	engine := signature.New()
	out, ok := engine.Signature([]byte(code), types.Kotlin)
	if !ok {
		t.Fatalf("expected ok=true for Kotlin signatures")
	}

	// 1. Verify package and imports are present
	if !strings.Contains(out, "package com.example.ui") {
		t.Errorf("expected package header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "import com.example.models.User as AppUser") {
		t.Errorf("expected import in output, got:\n%s", out)
	}

	// 2. Verify class compression and entity annotation
	if !strings.Contains(out, "@Entity(tableName = \"users\")\nclass DetailViewModel {") {
		t.Errorf("expected @Entity and class header, got:\n%s", out)
	}
	if strings.Contains(out, "heavy implementation") {
		t.Errorf("class method body was not compressed: found 'heavy implementation' in:\n%s", out)
	}
	if !strings.Contains(out, "  fun load() { /* ... */ }") {
		t.Errorf("expected compressed load() method, got:\n%s", out)
	}

	// 3. Verify single-expression functions
	if strings.Contains(out, "= true") || strings.Contains(out, "= \"OK\"") {
		t.Errorf("single expression function body was not stripped, got:\n%s", out)
	}
	if !strings.Contains(out, "  fun isReady(): Boolean { /* ... */ }") {
		t.Errorf("expected compressed isReady(), got:\n%s", out)
	}
	if !strings.Contains(out, "  fun checkStatus() { /* ... */ }") {
		t.Errorf("expected compressed checkStatus(), got:\n%s", out)
	}

	// 4. Verify custom getters and setters
	if strings.Contains(out, "return \"title\".uppercase()") {
		t.Errorf("getter body was not compressed, got:\n%s", out)
	}
	if !strings.Contains(out, "  get() { /* ... */ }") {
		t.Errorf("expected compressed get(), got:\n%s", out)
	}
	if !strings.Contains(out, "  set(value) { /* ... */ }") {
		t.Errorf("expected compressed set(value), got:\n%s", out)
	}
	// private set without body should be emitted verbatim without placeholder
	if !strings.Contains(out, "  private set") || strings.Contains(out, "private set { /* ... */ }") {
		t.Errorf("expected 'private set' without placeholder, got:\n%s", out)
	}

	// 5. Verify companion object compression
	if strings.Contains(out, "return DetailViewModel()") {
		t.Errorf("companion object body was not compressed, got:\n%s", out)
	}
	if !strings.Contains(out, "  companion object {") {
		t.Errorf("expected companion object container, got:\n%s", out)
	}
	if !strings.Contains(out, "    fun create(): DetailViewModel { /* ... */ }") {
		t.Errorf("expected nested method in companion object, got:\n%s", out)
	}

	// 6. Verify object_declaration compression
	if !strings.Contains(out, "object NetworkClient {") {
		t.Errorf("expected object NetworkClient container, got:\n%s", out)
	}
	if strings.Contains(out, "fetching") {
		t.Errorf("object method was not compressed, got:\n%s", out)
	}
	if !strings.Contains(out, "  fun fetch() { /* ... */ }") {
		t.Errorf("expected compressed fetch() in NetworkClient, got:\n%s", out)
	}

	// 7. Verify annotations on top-level function
	expectedFn := "@Composable\n@Preview\nfun DetailScreen(user: AppUser) { /* ... */ }"
	if !strings.Contains(out, expectedFn) {
		t.Errorf("expected annotated top-level function:\n%s\ngot:\n%s", expectedFn, out)
	}
}

func TestKotlinClassification(t *testing.T) {
	d := otherDriver{}

	cases := []struct {
		path     string
		file     string
		wantRole types.Role
		wantAdj  float64
		wantConf float64
	}{
		// Gradle Kotlin DSL
		{"build.gradle.kts", "build.gradle.kts", types.RoleConfig, 0.12, 0.95},
		{"settings.gradle.kts", "settings.gradle.kts", types.RoleConfig, 0.12, 0.95},
		{"app/build.gradle.kts", "build.gradle.kts", types.RoleConfig, 0.12, 0.95},

		// Presentation & State
		{"app/src/main/kotlin/HomeScreen.kt", "HomeScreen.kt", types.RoleImpl, 0.10, 0.85},
		{"app/src/main/kotlin/UserViewModel.kt", "UserViewModel.kt", types.RoleImpl, 0.10, 0.85},
		{"app/src/main/kotlin/MainActivity.kt", "MainActivity.kt", types.RoleImpl, 0.10, 0.85},
		{"app/src/main/kotlin/ConfirmDialog.kt", "ConfirmDialog.kt", types.RoleImpl, 0.10, 0.85},
		{"app/src/main/kotlin/ShareBottomSheet.kt", "ShareBottomSheet.kt", types.RoleImpl, 0.10, 0.85},

		// Data & Persistence
		{"core/data/UserRepository.kt", "UserRepository.kt", types.RoleAPI, 0.10, 0.85},
		{"core/database/UserDao.kt", "UserDao.kt", types.RoleAPI, 0.10, 0.85},
		{"core/database/AppDatabase.kt", "AppDatabase.kt", types.RoleAPI, 0.10, 0.85},
		{"core/model/UserEntity.kt", "UserEntity.kt", types.RoleSchema, 0.08, 0.85},

		// Tests
		{"app/src/test/kotlin/UserTest.kt", "UserTest.kt", types.RoleTest, -0.12, 0.90},
		{"app/src/androidTest/kotlin/UserUiTest.kt", "UserUiTest.kt", types.RoleTest, -0.12, 0.90},
		{"app/src/testDebug/kotlin/DebugLogicTest.kt", "DebugLogicTest.kt", types.RoleTest, -0.12, 0.90},
		{"app/src/testRelease/kotlin/ReleaseLogicTest.kt", "ReleaseLogicTest.kt", types.RoleTest, -0.12, 0.90},
		{"shared/src/commonTest/kotlin/CommonLogicTest.kt", "CommonLogicTest.kt", types.RoleTest, -0.12, 0.90},
		{"shared/src/androidUnitTest/kotlin/UnitTest.kt", "UnitTest.kt", types.RoleTest, -0.12, 0.90},
	}

	for _, tc := range cases {
		c := d.Classify(tc.path, tc.file)
		if c.Role != tc.wantRole {
			t.Errorf("Classify(%q, %q).Role = %v; want %v", tc.path, tc.file, c.Role, tc.wantRole)
		}
		if c.Adjustment != tc.wantAdj {
			t.Errorf("Classify(%q, %q).Adjustment = %v; want %v", tc.path, tc.file, c.Adjustment, tc.wantAdj)
		}
		if c.Confidence != tc.wantConf {
			t.Errorf("Classify(%q, %q).Confidence = %v; want %v", tc.path, tc.file, c.Confidence, tc.wantConf)
		}
	}
}

func TestKotlinImportResolution(t *testing.T) {
	d := otherDriver{}

	content := []byte(`package com.example.ui.screens

import com.example.models.User as AppUser
import com.example.models.Status.ACTIVE
import com.example.util.formatDate
import com.example.common.*
`)

	// Standard anchored package:
	// dir: app/src/main/kotlin/com/example/ui/screens
	// package: com.example.ui.screens
	// source root: app/src/main/kotlin
	targets := d.Imports("app/src/main/kotlin/com/example/ui/screens/HomeScreen.kt", "", content)

	hasTarget := func(wanted string) bool {
		for _, target := range targets {
			if target == wanted {
				return true
			}
		}
		return false
	}

	// 1. Aliased import should resolve target package
	if !hasTarget("app/src/main/kotlin/com/example/models") {
		t.Errorf("expected package target for models from User as AppUser, got %v", targets)
	}

	// 2. Companion / Enum member import Status.ACTIVE should target models package (dropping both ACTIVE and Status)
	if !hasTarget("app/src/main/kotlin/com/example/models") {
		t.Errorf("expected package target for models from Status.ACTIVE, got %v", targets)
	}
	// It should NOT treat Status as a package subdirectory
	if hasTarget("app/src/main/kotlin/com/example/models/Status") {
		t.Errorf("expected Status to NOT be treated as a package subdirectory, got %v", targets)
	}

	// 3. Top-level function import should target util package
	if !hasTarget("app/src/main/kotlin/com/example/util") {
		t.Errorf("expected target for util package from formatDate, got %v", targets)
	}

	// 4. Wildcard import
	if !hasTarget("app/src/main/kotlin/com/example/common") {
		t.Errorf("expected target for common wildcard, got %v", targets)
	}

	// 5. Decoupled KMP package:
	// dir: shared/src/commonMain/kotlin
	// package: com.example.ui.screens (diverged from physical directory)
	kmpTargets := d.Imports("shared/src/commonMain/kotlin/HomeScreen.kt", "", content)
	hasKmpTarget := func(wanted string) bool {
		for _, target := range kmpTargets {
			if target == wanted {
				return true
			}
		}
		return false
	}
	if !hasKmpTarget("shared/src/commonMain/kotlin/com/example/models") {
		t.Errorf("expected KMP target for models, got %v", kmpTargets)
	}
	if !hasKmpTarget("shared/src/commonMain/kotlin") {
		t.Errorf("expected KMP fallback target for source root, got %v", kmpTargets)
	}
}
