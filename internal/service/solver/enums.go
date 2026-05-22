package solver

import solverv1 "github.com/dnonakolesax/cccad-locks/internal/proto/solver/v1"

func constraintStatus(status string) solverv1.ConstraintStatus {
	switch status {
	case "active":
		return solverv1.ConstraintStatus_CONSTRAINT_STATUS_ACTIVE
	case "suppressed":
		return solverv1.ConstraintStatus_CONSTRAINT_STATUS_SUPPRESSED
	case "invalid":
		return solverv1.ConstraintStatus_CONSTRAINT_STATUS_INVALID
	case "deleted":
		return solverv1.ConstraintStatus_CONSTRAINT_STATUS_DELETED
	default:
		return solverv1.ConstraintStatus_CONSTRAINT_STATUS_UNSPECIFIED
	}
}

func tangentBranch(branch string) solverv1.TangentBranch {
	switch branch {
	case "external":
		return solverv1.TangentBranch_TANGENT_BRANCH_EXTERNAL
	case "internal":
		return solverv1.TangentBranch_TANGENT_BRANCH_INTERNAL
	default:
		return solverv1.TangentBranch_TANGENT_BRANCH_UNSPECIFIED
	}
}

func arcBranch(branch string) solverv1.ArcBranch {
	switch branch {
	case "minor":
		return solverv1.ArcBranch_ARC_BRANCH_MINOR
	case "major":
		return solverv1.ArcBranch_ARC_BRANCH_MAJOR
	default:
		return solverv1.ArcBranch_ARC_BRANCH_UNSPECIFIED
	}
}

func arcBranchString(branch solverv1.ArcBranch) string {
	switch branch {
	case solverv1.ArcBranch_ARC_BRANCH_MINOR:
		return "minor"
	case solverv1.ArcBranch_ARC_BRANCH_MAJOR:
		return "major"
	default:
		return "unspecified"
	}
}

func equalKind(kind string) solverv1.EqualKind {
	switch kind {
	case "length":
		return solverv1.EqualKind_EQUAL_KIND_LENGTH
	case "radius":
		return solverv1.EqualKind_EQUAL_KIND_RADIUS
	default:
		return solverv1.EqualKind_EQUAL_KIND_UNSPECIFIED
	}
}

func distanceReferenceKind(kind string) solverv1.DistanceReferenceKind {
	switch kind {
	case "point_point":
		return solverv1.DistanceReferenceKind_DISTANCE_REFERENCE_KIND_POINT_POINT
	case "point_line":
		return solverv1.DistanceReferenceKind_DISTANCE_REFERENCE_KIND_POINT_LINE
	case "line_line":
		return solverv1.DistanceReferenceKind_DISTANCE_REFERENCE_KIND_LINE_LINE
	default:
		return solverv1.DistanceReferenceKind_DISTANCE_REFERENCE_KIND_UNSPECIFIED
	}
}

func angleOrientation(orientation string) solverv1.AngleOrientation {
	switch orientation {
	case "cw":
		return solverv1.AngleOrientation_ANGLE_ORIENTATION_CW
	case "ccw":
		return solverv1.AngleOrientation_ANGLE_ORIENTATION_CCW
	default:
		return solverv1.AngleOrientation_ANGLE_ORIENTATION_UNSPECIFIED
	}
}

func solveStatusString(status solverv1.SolveStatus) string {
	switch status {
	case solverv1.SolveStatus_SOLVE_STATUS_OK:
		return "ok"
	case solverv1.SolveStatus_SOLVE_STATUS_UNDER_CONSTRAINED:
		return "under_constrained"
	case solverv1.SolveStatus_SOLVE_STATUS_FULLY_CONSTRAINED:
		return "fully_constrained"
	case solverv1.SolveStatus_SOLVE_STATUS_OVER_CONSTRAINED:
		return "over_constrained"
	case solverv1.SolveStatus_SOLVE_STATUS_INCONSISTENT:
		return "inconsistent"
	case solverv1.SolveStatus_SOLVE_STATUS_NUMERICAL_FAILURE:
		return "numerical_failure"
	default:
		return "unspecified"
	}
}

func diagnosticLevelString(level solverv1.SolverDiagnosticLevel) string {
	switch level {
	case solverv1.SolverDiagnosticLevel_SOLVER_DIAGNOSTIC_LEVEL_INFO:
		return "info"
	case solverv1.SolverDiagnosticLevel_SOLVER_DIAGNOSTIC_LEVEL_WARNING:
		return "warning"
	case solverv1.SolverDiagnosticLevel_SOLVER_DIAGNOSTIC_LEVEL_ERROR:
		return "error"
	default:
		return "unspecified"
	}
}
