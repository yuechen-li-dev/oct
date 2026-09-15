module basic_fsm_tb;
    logic Clock = 0;
    logic Reset = 1;
    logic signed [63:0] limit = 3;
    logic Turn_enable = 1;
    logic Done, Suspended, Fault, YieldValid;
    logic [0:0] StateView;
    logic [1:0] InstructionView;
    logic signed [63:0] Result, YieldValue, Board_Count;
    logic Board_Armed;
    logic WaitDone, WaitSuspended, WaitFault;
    logic [0:0] WaitStateView;
    logic [1:0] WaitInstructionView;
    logic signed [63:0] WaitResult;

    Counter dut(.*);
    WaitOnce wait_dut(
        .Clock(Clock), .Reset(Reset), .Done(WaitDone),
        .Suspended(WaitSuspended), .Fault(WaitFault),
        .StateView(WaitStateView), .InstructionView(WaitInstructionView),
        .Result(WaitResult)
    );

    always #5 Clock = ~Clock;

    initial begin
        @(posedge Clock); #1; Reset = 0;
        @(posedge Clock); #1;
        if (Fault || Done || !YieldValid || YieldValue != 1 || Board_Count != 1 || !Board_Armed) $fatal(1, "counter turn 1 mismatch");
        if (WaitFault || WaitDone || !WaitSuspended) $fatal(1, "suspend turn mismatch");
        @(posedge Clock); #1;
        if (Fault || Done || !YieldValid || YieldValue != 2 || Board_Count != 2) $fatal(1, "counter turn 2 mismatch");
        if (WaitFault || !WaitDone || WaitSuspended || WaitResult != 9) $fatal(1, "suspend continuation mismatch");
        @(posedge Clock); #1;
        if (Fault || !Done || YieldValid || Result != 3 || Board_Count != 3) $fatal(1, "counter completion mismatch");
        $display("basic-fsm-equivalence-ok");
        $finish;
    end
endmodule
