module utility_standalone_tb;
    logic Clock = 0;
    logic Reset = 1;
    logic [129:0] Turn_scores;
    logic Done, Suspended, Fault, YieldValid;
    logic [0:0] StateView;
    logic [2:0] InstructionView;
    logic signed [63:0] Result, YieldValue, Board_Selected;

    UtilityChooser dut(.*);

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

    // A standalone `when utility` keeps no commitment: every turn is decided
    // by that turn's conditions and scores alone.
    task expect_yield(input signed [63:0] expected);
        begin
            @(posedge Clock); #1;
            if (Fault || Done || !YieldValid || YieldValue != expected || Board_Selected != expected) $fatal(1, "utility turn mismatch");
        end
    endtask

    initial begin
        set_scores(1, 1, 10, 10);
        @(posedge Clock); #1; Reset = 0;
        expect_yield(1);
        set_scores(1, 1, 10, 11); expect_yield(2);
        set_scores(1, 1, 12, 11); expect_yield(1);
        set_scores(0, 1, 99, 1); expect_yield(2);
        set_scores(0, 0, 5, 5); expect_yield(0);
        $display("utility-standalone-equivalence-ok");
        $finish;
    end
endmodule
