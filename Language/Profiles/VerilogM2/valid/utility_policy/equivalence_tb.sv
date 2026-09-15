module utility_policy_tb;
    logic Clock = 0;
    logic Reset = 1;
    logic [129:0] Turn_scores;
    logic Done, Suspended, Fault, YieldValid;
    logic [0:0] StateView;
    logic [2:0] InstructionView;
    logic signed [63:0] Result, YieldValue, Board_Selected;
    logic UtilitySite0HasCurrent;
    logic signed [63:0] UtilitySite0Current, UtilitySite0Score, UtilitySite0CommitAge;

    UtilityController dut(.*);

    always #5 Clock = ~Clock;

    task set_scores(input logic av, input logic bv, input signed [63:0] ascore, input signed [63:0] bscore);
        begin
            Turn_scores = '0;
            Turn_scores[0] = av;
            Turn_scores[1] = bv;
            Turn_scores[65:2] = ascore;
            Turn_scores[129:66] = bscore;
        end
    endtask

    task expect_yield(input signed [63:0] expected, input signed [63:0] age);
        begin
            @(posedge Clock); #1;
            if (Fault || Done || !YieldValid || YieldValue != expected || Board_Selected != expected || UtilitySite0Current != expected || UtilitySite0CommitAge != age) $fatal(1, "utility turn mismatch");
        end
    endtask

    initial begin
        set_scores(1, 1, 10, 10);
        @(posedge Clock); #1; Reset = 0;
        expect_yield(1, 1);
        set_scores(1, 1, 10, 11); expect_yield(1, 2);
        set_scores(1, 1, 10, 20); expect_yield(1, 3);
        set_scores(1, 1, 10, 20); expect_yield(2, 1);
        set_scores(0, 0, 0, 0); expect_yield(0, 1);
        $display("utility-policy-equivalence-ok");
        $finish;
    end
endmodule
