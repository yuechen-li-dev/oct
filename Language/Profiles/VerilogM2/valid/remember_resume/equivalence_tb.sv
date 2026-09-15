module remember_resume_tb;
    logic Clock = 0;
    logic Reset = 1;
    logic Turn_divert = 1;
    logic Done, Suspended, Fault, YieldValid, HasResumeTarget;
    logic [1:0] StateView, ResumeStateView;
    logic [1:0] InstructionView;
    logic signed [63:0] Result, YieldValue;

    Interruptible dut(.*);

    always #5 Clock = ~Clock;

    initial begin
        @(posedge Clock); #1; Reset = 0;
        @(posedge Clock); #1;
        if (Fault || Done || !YieldValid || YieldValue != 2 || !HasResumeTarget) $fatal(1, "remember turn mismatch");
        Turn_divert = 0;
        @(posedge Clock); #1;
        if (Fault || Done || !YieldValid || YieldValue != 1 || HasResumeTarget) $fatal(1, "resume turn mismatch");
        $display("remember-resume-equivalence-ok");
        $finish;
    end
endmodule
